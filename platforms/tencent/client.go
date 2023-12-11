package tencent

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/csh995426531/asr-client/enums"
	"github.com/csh995426531/asr-client/platforms"
	"github.com/gorilla/websocket"
	"io"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type Conf struct {
	Enable          bool   `json:"enable"`
	HostURL         string `json:"host_url"`
	APPID           string `json:"appid"`
	SecretID        string `json:"secret_id"`
	SecretKey       string `json:"secret_key"`
	EngineModelType string `json:"engine_model_type"`
}

type Tencent struct {
	cfg *Conf
}

const (
	defaultVoiceFormat       = 1 // 1：pcm；4：speex(sp)；6：silk；8：mp3；10：opus（opus 格式音频流封装说明）；12：wav；14：m4a（每个分片须是一个完整的 m4a 音频）；16：aac
	defaultNeedVad           = 1
	defaultWordInfo          = 0
	defaultFilterDirty       = 0
	defaultFilterModal       = 0
	defaultFilterPunc        = 0
	defaultConvertNumMode    = 1
	defaultReinforceHotword  = 0
	defaultFilterEmptyResult = 1
	defaultMaxSpeakTime      = 0

	frameSize = 6400
	protocol  = "wss"
)

const (
	eventTypeRecognitionStart        = 0
	eventTypeSentenceBegin           = 1
	eventTypeRecognitionResultChange = 2
	eventTypeSentenceEnd             = 3
	eventTypeRecognitionComplete     = 4
	eventTypeFail                    = 5
)

func NewTencent(c *Conf) *Tencent {
	return &Tencent{
		cfg: c,
	}
}

func (t *Tencent) getInterval() time.Duration {
	return 20 * time.Millisecond //发送音频间隔
}

func (t *Tencent) Conn() (platforms.Cli, error) {
	c := &Client{
		Tencent: t,
		voiceID: uuid.New().String(),
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

func (t *Tencent) Info() enums.Platform {
	return enums.Tencent
}

func (t *Tencent) genSignature(url string) string {
	mac := hmac.New(sha1.New, []byte(t.cfg.SecretKey))
	signURL := url
	mac.Write([]byte(signURL))
	encryptedStr := mac.Sum([]byte(nil))
	var signature = base64.StdEncoding.EncodeToString(encryptedStr)

	return signature
}

// SpeechRecognitionResponse is the reponse of asr service
type SpeechRecognitionResponse struct {
	Code      int                             `json:"code"`
	Message   string                          `json:"message"`
	VoiceID   string                          `json:"voice_id,omitempty"`
	MessageID string                          `json:"message_id,omitempty"`
	Final     uint32                          `json:"final,omitempty"`
	Result    SpeechRecognitionResponseResult `json:"result,omitempty"`
}

// SpeechRecognitionResponseResult .
type SpeechRecognitionResponseResult struct {
	SliceType    uint32                                `json:"slice_type"`
	Index        int                                   `json:"index"`
	StartTime    uint32                                `json:"start_time"`
	EndTime      uint32                                `json:"end_time"`
	VoiceTextStr string                                `json:"voice_text_str"`
	WordSize     uint32                                `json:"word_size"`
	WordList     []SpeechRecognitionResponseResultWord `json:"word_list"`
}

// SpeechRecognitionResponseResultWord .
type SpeechRecognitionResponseResultWord struct {
	Word       string `json:"word"`
	StartTime  uint32 `json:"start_time"`
	EndTime    uint32 `json:"end_time"`
	StableFlag uint32 `json:"stable_flag"`
}

type Client struct {
	*Tencent
	conn    *websocket.Conn
	voiceID string
}

func (c *Client) createWebSocket() (*websocket.Conn, error) {
	d := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}
	//握手并建立websocket 连接
	conn, resp, err := d.Dial(c.assembleAuthUrl(), nil)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != 101 {
		return nil, err
	}
	return conn, nil
}

func (c *Client) Send(file io.ReadCloser) (err error) {
	var buffer = make([]byte, frameSize)
	for {
		readLen, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF { //文件读取完了
				break
			} else {
				return err
			}
		}
		if readLen <= 0 {
			break
		}
		if err = c.conn.WriteMessage(websocket.BinaryMessage, buffer); err != nil {
			return err
		}
		time.Sleep(c.getInterval())
	}
	if err = c.conn.WriteMessage(websocket.TextMessage, []byte("{\"type\":\"end\"}")); err != nil {
		return err
	}
	return nil
}

func (c *Client) Receive() (string, error) {
	defer c.conn.Close()
	var result string
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return "", err
		}

		data := SpeechRecognitionResponse{}
		err = json.Unmarshal(msg, &data)
		//fmt.Printf("%+v\r\n", data)
		if err != nil {
			return "", err
		}
		if data.Code != 0 {
			fmt.Printf("VoiceID: %s, error code %d, message: %s\n",
				c.voiceID, data.Code, data.Message)
			break
		}

		if data.Final == 1 {
			break
		}
		result = data.Result.VoiceTextStr
	}
	return result, nil
}

func (c *Client) assembleAuthUrl() string {
	var queryMap = make(map[string]string)
	queryMap["secretid"] = c.cfg.SecretID
	var timestamp = time.Now().Unix()
	var timestampStr = strconv.FormatInt(timestamp, 10)
	queryMap["timestamp"] = timestampStr
	queryMap["expired"] = strconv.FormatInt(timestamp+24*60*60, 10)
	queryMap["nonce"] = timestampStr

	//params
	queryMap["engine_model_type"] = c.cfg.EngineModelType
	queryMap["voice_id"] = c.voiceID
	queryMap["voice_format"] = strconv.FormatInt(int64(defaultVoiceFormat), 10)
	queryMap["needvad"] = strconv.FormatInt(int64(defaultNeedVad), 10)
	//if recognizer.HotwordId != "" {
	//	queryMap["hotword_id"] = recognizer.HotwordId
	//}
	//if recognizer.HotwordList != "" {
	//	queryMap["hotword_list"] = recognizer.HotwordList
	//}
	//if recognizer.CustomizationId != "" {
	//	queryMap["customization_id"] = recognizer.CustomizationId
	//}
	queryMap["filter_dirty"] = strconv.FormatInt(int64(defaultFilterDirty), 10)
	queryMap["filter_modal"] = strconv.FormatInt(int64(defaultFilterModal), 10)
	queryMap["filter_punc"] = strconv.FormatInt(int64(defaultFilterPunc), 10)
	queryMap["filter_empty_result"] = strconv.FormatInt(int64(defaultFilterEmptyResult), 10)
	queryMap["convert_num_mode"] = strconv.FormatInt(int64(defaultConvertNumMode), 10)
	queryMap["word_info"] = strconv.FormatInt(int64(defaultWordInfo), 10)
	queryMap["reinforce_hotword"] = strconv.FormatInt(int64(defaultReinforceHotword), 10)
	queryMap["max_speak_time"] = strconv.FormatInt(int64(defaultMaxSpeakTime), 10)
	//if recognizer.VadSilenceTime > 0 {
	//	queryMap["vad_silence_time"] = strconv.FormatInt(int64(recognizer.VadSilenceTime), 10)
	//}
	//if recognizer.NoiseThreshold != 0 {
	//	queryMap["noise_threshold"] = strconv.FormatFloat(recognizer.NoiseThreshold, 'f', 3, 64)
	//}

	var keys []string
	for k := range queryMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var queryStrBuffer bytes.Buffer
	for _, k := range keys {
		queryStrBuffer.WriteString(k)
		queryStrBuffer.WriteString("=")
		queryStrBuffer.WriteString(queryMap[k])
		queryStrBuffer.WriteString("&")
	}

	rs := []rune(queryStrBuffer.String())
	rsLen := len(rs)
	queryStr := string(rs[0 : rsLen-1])

	//gen url
	curlUrl := fmt.Sprintf("%s/asr/v2/%s?%s", c.cfg.HostURL, c.cfg.APPID, queryStr)
	signature := c.genSignature(curlUrl)

	return fmt.Sprintf("%s://%s&signature=%s", protocol, curlUrl, url.QueryEscape(signature))
}
