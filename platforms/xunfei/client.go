package xunfei

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/csh995426531/asr-client/enums"
	"github.com/csh995426531/asr-client/platforms"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Conf struct {
	Enable    bool   `json:"enable"`
	HostURL   string `json:"host_url"`
	APPID     string `json:"appid"`
	APISecret string `json:"api_secret"`
	APIKey    string `json:"api_key"`
}

type XunFei struct {
	cfg *Conf
}

var (
	frameSize = 1280
)

func NewXunFei(c *Conf) *XunFei {
	return &XunFei{
		cfg: c,
	}
}

func (x *XunFei) getInterval() time.Duration {
	return 20 * time.Millisecond //发送音频间隔
}

func (x *XunFei) Conn() (platforms.Cli, error) {
	c := &Client{
		XunFei: x,
	}
	var err error
	if c.conn, err = c.createWebSocket(); err != nil {
		c.conn = nil
		fmt.Printf("create WebSocket fail : %v", err)
	}
	if c.conn == nil {
		return nil, errors.New("create WebSocket fail")
	}
	return c, nil
}

func (x *XunFei) Info() enums.Platform {
	return enums.XunFei
}

func (x *XunFei) createWebSocket() (*websocket.Conn, error) {
	d := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}
	//握手并建立websocket 连接
	conn, resp, err := d.Dial(x.assembleAuthUrl(), nil)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != 101 {
		return nil, err
	}
	return conn, nil
}

func (x *XunFei) buildFrame(status enums.FileStatus, buffer []byte, len int) map[string]interface{} {
	var frameData = make(map[string]interface{})
	if status == enums.FirstFrame {
		frameData = map[string]interface{}{
			"common": map[string]interface{}{
				"app_id": x.cfg.APPID, //appid 必须带上，只需第一帧发送
			},
			"business": map[string]interface{}{ //business 参数，只需一帧发送
				"language": "zh_cn",
				"domain":   "iat",
				"accent":   "mandarin",
			},
		}
		fmt.Println("send first")
	}

	frameData["data"] = map[string]interface{}{
		"status":   status.Value,
		"format":   "audio/L16;rate=16000",
		"audio":    base64.StdEncoding.EncodeToString(buffer[:len]),
		"encoding": "raw",
	}
	return frameData
}

func (x *XunFei) extractData(data []byte) string {
	var resp = RespData{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return ""
	}
	fmt.Println(resp.Data.Result.String(), resp.Sid)
	st := time.Now()
	if resp.Code != 0 {
		fmt.Println(resp.Code, resp.Message, time.Since(st))
		return ""
	}
	//decoder.Decode(&resp.Data.Result)
	if resp.Data.Status == 2 {
		//cf()
		//fmt.Println("final:",decoder.String())
		fmt.Println(resp.Code, resp.Message, time.Since(st))
		return resp.Data.Result.String()
	}
	return ""
}

type RespData struct {
	Sid     string `json:"sid"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    Data   `json:"data"`
}

type Data struct {
	Result Result `json:"result"`
	Status int    `json:"status"`
}

// 创建鉴权url  apikey 即 hmac username
func (x *XunFei) assembleAuthUrl() string {
	ul, err := url.Parse(x.cfg.HostURL)
	if err != nil {
		fmt.Println(err)
	}
	//签名时间
	date := time.Now().UTC().Format(time.RFC1123)
	//date = "Tue, 28 May 2019 09:10:42 MST"
	//参与签名的字段 host ,date, request-line
	signString := []string{"host: " + ul.Host, "date: " + date, "GET " + ul.Path + " HTTP/1.1"}
	//拼接签名字符串
	sgin := strings.Join(signString, "\n")
	fmt.Println(sgin)
	//签名结果
	sha := HmacWithShaToBase64("hmac-sha256", sgin, x.cfg.APISecret)
	fmt.Println(sha)
	//构建请求参数 此时不需要urlencoding
	authUrl := fmt.Sprintf("hmac username=\"%s\", algorithm=\"%s\", headers=\"%s\", signature=\"%s\"",
		x.cfg.APIKey, "hmac-sha256", "host date request-line", sha)
	//将请求参数使用base64编码
	authorization := base64.StdEncoding.EncodeToString([]byte(authUrl))

	v := url.Values{}
	v.Add("host", ul.Host)
	v.Add("date", date)
	v.Add("authorization", authorization)
	//将编码后的字符串url encode后添加到url后面
	callurl := x.cfg.HostURL + "?" + v.Encode()
	return callurl
}

func HmacWithShaToBase64(algorithm, data, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	encodeData := mac.Sum(nil)
	return base64.StdEncoding.EncodeToString(encodeData)
}

func readResp(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("code=%d,body=%s", resp.StatusCode, string(b))
}

type Client struct {
	*XunFei
	conn *websocket.Conn
}

func (c *Client) Send(file io.ReadCloser) (err error) {

	var status = enums.FirstFrame
	var buffer = make([]byte, frameSize)
	for {
		readLen, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF { //文件读取完了，改变status = StatusLastFrame
				status = enums.LastFrame
			} else {
				fmt.Printf("read file error: %v\n", err)
				return nil
			}
		}

		frameData := c.buildFrame(status, buffer, readLen)
		if status == enums.FirstFrame {
			status = enums.ContinueFrame
		}
		if err = c.conn.WriteJSON(frameData); err != nil {
			return err
		}
		if status == enums.LastFrame {
			fmt.Println("send last")
			return nil
		}
		time.Sleep(c.getInterval())
	}
}

func (c *Client) Receive() (string, error) {
	defer c.conn.Close()
	//获取返回的数据
	var result string
	_, msg, err := c.conn.ReadMessage()
	if err != nil {
		return "", err
	}
	result = c.extractData(msg)
	return result, nil
}

// 解析返回数据，仅供demo参考，实际场景可能与此不同。
type Decoder struct {
	results []*Result
}

func (d *Decoder) Decode(result *Result) {
	if len(d.results) <= result.Sn {
		d.results = append(d.results, make([]*Result, result.Sn-len(d.results)+1)...)
	}
	if result.Pgs == "rpl" {
		for i := result.Rg[0]; i <= result.Rg[1]; i++ {
			d.results[i] = nil
		}
	}
	d.results[result.Sn] = result
}

func (d *Decoder) String() string {
	var r string
	for _, v := range d.results {
		if v == nil {
			continue
		}
		r += v.String()
	}
	return r
}

type Result struct {
	Ls  bool   `json:"ls"`
	Rg  []int  `json:"rg"`
	Sn  int    `json:"sn"`
	Pgs string `json:"pgs"`
	Ws  []Ws   `json:"ws"`
}

func (t *Result) String() string {
	var wss string
	for _, v := range t.Ws {
		wss += v.String()
	}
	return wss
}

type Ws struct {
	Bg int  `json:"bg"`
	Cw []Cw `json:"cw"`
}

func (w *Ws) String() string {
	var wss string
	for _, v := range w.Cw {
		wss += v.W
	}
	return wss
}

type Cw struct {
	Sc int    `json:"sc"`
	W  string `json:"w"`
}
