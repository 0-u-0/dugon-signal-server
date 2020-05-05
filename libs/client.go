package libs

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"log"
	"math/rand"
	"reflect"
	"time"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 2048 //TODO: adjust
)

type jsonMap = map[string]interface{}

type Client struct {
	clientGroup *ClientGroup
	tokenId     string
	sessionId   string
	metadata    map[string]string
	isPub       bool
	isSub       bool
	conn        *websocket.Conn
	send        chan []byte
	recv        chan []byte
	selfSub     *nats.Subscription
	sessionSub  *nats.Subscription
	//FIXME: maybe pub,sub use different mediaserver
	mediaServer *MediaServer
}

type requestParams struct {
	SessionId string            `json:"sessionId"`
	TokenId   string            `json:"tokenId"`
	Metadata  map[string]string `json:"metadata"`
}

//TODO(CC): use random id
func (c *Client) idGenerator(role string) string {
	data := []byte(fmt.Sprintf("%s@%s@%s", c.sessionId, c.tokenId, role))
	has := md5.Sum(data)
	return fmt.Sprintf("%x", has)
}

func (c *Client) sendJson(v interface{}) {
	if err := c.conn.WriteJSON(v); err != nil {
		fmt.Println(err)
		//TODO:
	}
}

func (c *Client) responseClient(id int, params interface{}) {
	response := jsonMap{
		"method": "response",
		"id":     id,
		"params": params,
	}
	c.sendJson(response)
}

func (c *Client) notification(event string, data interface{}) {
	response := jsonMap{
		"method": "notification",
		"params": jsonMap{
			"event": event,
			"data":  data,
		},
	}
	c.sendJson(response)
}

func (c *Client) responseClientWithoutData(id int) {
	c.responseClient(id, map[string]string{})
}

func (c *Client) setMediaServer() {
	//
	if len(c.clientGroup.mediaServers) > 0 {
		if c.mediaServer == nil {
			rand.Seed(time.Now().Unix())
			keys := reflect.ValueOf(c.clientGroup.mediaServers).MapKeys()
			randId := keys[rand.Intn(len(keys))].String()
			ms, ok := c.clientGroup.mediaServers[randId]
			if ok {
				c.mediaServer = ms
			}
		}
	} else {
		//TODO(CC): no media server
	}
}

func (c *Client) handleMessage(message []byte) {
	var requestMes *RequestMessage
	jsonErr := json.Unmarshal(message, &requestMes)
	if jsonErr != nil {
		//TODO(CC):
		return
	}

	fmt.Println(len(message))
	if requestMes.Method == "request" {
		data := requestMes.Params.Data
		switch requestMes.Params.Event {
		case "join":
			pub := requestMes.Params.Data["pub"].(bool)
			sub := requestMes.Params.Data["sub"].(bool)

			//FIXME: ...
			c.setMediaServer()

			codecData := c.requestMediaNoParams("codecs")

			responseParams := jsonMap{
				"codecs": codecData["codecs"],
			}

			if pub {
				c.isPub = true
				transportId := c.idGenerator("pub")

				mediaRequest := jsonMap{
					"transportId": transportId,
					"role":        "pub",
				}
				pubData := c.requestMedia("transport", mediaRequest)
				responseParams["pub"] = pubData["transportParameters"]
			}

			if sub {
				c.isSub = true
				transportId := c.idGenerator("sub")

				mediaRequest := map[string]interface{}{
					"transportId": transportId,
					"role":        "sub",
				}
				pubData := c.requestMedia("transport", mediaRequest)
				responseParams["sub"] = pubData["transportParameters"]
			}

			c.responseClient(requestMes.Id, responseParams)

			c.publish2Session("join", jsonMap{
				"metadata": c.metadata,
				"pub":      c.isPub,
				"sub":      c.isSub,
			})

			c.subscribeNATS()

		case "dtls":
			c.requestMedia("dtls", jsonMap{
				"transportId":    requestMes.Params.Data["transportId"],
				"dtlsParameters": requestMes.Params.Data["dtlsParameters"],
			})
			c.responseClientWithoutData(requestMes.Id)
		case "publish":
			senderData := c.requestMedia("publish", jsonMap{
				"transportId": requestMes.Params.Data["transportId"],
				"codec":       requestMes.Params.Data["codec"],
				"metadata":    requestMes.Params.Data["metadata"],
			})
			c.responseClient(requestMes.Id, jsonMap{
				"senderId": senderData["senderId"],
			})

			c.publish2Session("publish", jsonMap{
				"senderId": senderData["senderId"],
				"metadata": requestMes.Params.Data["metadata"],
			})
		case "unpublish":
			c.requestMedia("unpublish", jsonMap{
				"transportId": data["transportId"],
				"senderId":    data["senderId"],
			})
			c.responseClientWithoutData(requestMes.Id)

			c.publish2Session("unpublish", jsonMap{
				"senderId": data["senderId"],
			})
		//case "subscribe":
		case "unsubscribe":
			c.requestMedia("unsubscribe", jsonMap{
				"transportId": data["transportId"],
				"senderId":    data["senderId"],
			})
			c.responseClientWithoutData(requestMes.Id)
		case "pause":
			c.requestMedia("pause", jsonMap{
				"transportId": data["transportId"],
				"senderId":    data["senderId"],
			})
			c.responseClientWithoutData(requestMes.Id)
			if data["role"].(string) == "pub" {
				c.publish2Session("pause", jsonMap{
					"senderId": data["senderId"],
				})
			}
		case "resume":
			c.requestMedia("resume", jsonMap{
				"transportId": data["transportId"],
				"senderId":    data["senderId"],
			})
			c.responseClientWithoutData(requestMes.Id)
			if data["role"].(string) == "pub" {
				c.publish2Session("resume", jsonMap{
					"senderId": data["senderId"],
				})
			}
		}

	}
}

type MediaRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

type MediaResponse struct {
	Method string  `json:"method"`
	Data   jsonMap `json:"data"`
}

//NATS
//---------------------
func (c *Client) publish2Session(method string, data jsonMap) {
	sessionSubject := fmt.Sprintf("%s.@", c.sessionId)

	c.clientGroup.nc.Publish(sessionSubject, map[string]interface{}{
		"tokenId": c.tokenId,
		"method":  method,
		"data":    data,
	})
}

func (c *Client) publish2One(tokenId string, method string, data jsonMap) {
	oneSubject := fmt.Sprintf("%s.%s", c.sessionId, tokenId)

	c.clientGroup.nc.Publish(oneSubject, map[string]interface{}{
		"tokenId": c.tokenId,
		"method":  method,
		"data":    data,
	})
}

type natsSubscribedMessage struct {
	TokenId string  `json:"tokenId"`
	Method  string  `json:"method"`
	Data    jsonMap `json:"data"`
}

func (c *Client) notifySenders(tokenId string) {

	transportId := c.idGenerator("pub")
	sendersData := c.requestMedia("senders", jsonMap{
		"transportId": transportId,
	})
	senders := sendersData["senders"].([]interface{})

	for _, s := range senders {
		sender := s.(jsonMap)
		c.publish2One(tokenId, "publish", jsonMap{
			"senderId": sender["id"],
			"metadata": sender["metadata"],
		})
	}
}

func (c *Client) notifySender2Client(tokenId string, senderId string, metadata interface{}) {
	transportId := c.idGenerator("sub")
	subData := c.requestMedia("subscribe", jsonMap{
		"transportId": transportId,
		"senderId":    senderId,
	})

	c.notification("publish", jsonMap{
		"codec":      subData["codec"],
		"receiverId": subData["receiverId"],
		"senderId":   senderId,
		"tokenId":    tokenId,
		"metadata":   metadata,
	})

}

func (c *Client) subscribeNATS() {
	selfSubject := fmt.Sprintf("%s.%s", c.sessionId, c.tokenId)
	//TODO: error
	selfSub, _ := c.clientGroup.nc.Subscribe(selfSubject, func(m *nats.Msg) {
		fmt.Printf("Received a message: %s\n", string(m.Data))

		var msg natsSubscribedMessage
		err := json.Unmarshal(m.Data, &msg)
		if err != nil {
			//TODO(CC): error
			fmt.Println(err)
		}

		tokenId := msg.TokenId
		switch msg.Method {
		case "join":
			metadata := msg.Data["metadata"]
			sub := msg.Data["sub"].(bool)

			c.notification("join", jsonMap{
				"tokenId":  tokenId,
				"metadata": metadata,
			})

			c.publish2One(tokenId, "join", jsonMap{
				"metadata": c.metadata,
				"pub":      c.isPub,
				"sub":      c.isSub,
			})

			if c.isPub && sub {
				c.notifySenders(tokenId)
			}

		case "leave":
			c.notification("leave", jsonMap{
				"tokenId": tokenId,
			})
		case "publish":
			senderId := msg.Data["senderId"].(string)
			metadata := msg.Data["metadata"]
			c.notifySender2Client(tokenId, senderId, metadata)
		case "unpublish":
			c.notification("unpublish", jsonMap{
				"senderId": msg.Data["senderId"],
				"tokenId":  tokenId,
			})
		case "pause":
			c.notification("pause", jsonMap{
				"senderId": msg.Data["senderId"],
			})
		case "resume":
			c.notification("resume", jsonMap{
				"senderId": msg.Data["senderId"],
			})
		}

	})
	c.selfSub = selfSub

	sessionSubject := fmt.Sprintf("%s.@", c.sessionId)
	//TODO(CC): error
	sessionSub, _ := c.clientGroup.nc.Subscribe(sessionSubject, func(m *nats.Msg) {
		fmt.Printf("Received a session message: %s\n", string(m.Data))

		var msg natsSubscribedMessage
		err := json.Unmarshal(m.Data, &msg)
		if err != nil {
			//TODO(CC): error
			fmt.Println(err)
		}

		tokenId := msg.TokenId
		if tokenId != c.tokenId {
			switch msg.Method {
			case "join":
				metadata := msg.Data["metadata"]
				sub := msg.Data["sub"].(bool)

				c.notification("join", jsonMap{
					"tokenId":  tokenId,
					"metadata": metadata,
				})

				if c.isPub && sub {
					c.notifySenders(tokenId)
				}
			case "publish":
				senderId := msg.Data["senderId"].(string)
				metadata := msg.Data["metadata"]
				c.notifySender2Client(tokenId, senderId, metadata)

			}
		}

	})
	c.sessionSub = sessionSub
}

func (c *Client) requestMedia(method string, params jsonMap) jsonMap {

	request := MediaRequest{Method: method, Params: params}

	var response MediaResponse

	mediaSubject := fmt.Sprintf("media.%s", c.mediaServer.Id)
	err := c.clientGroup.nc.Request(mediaSubject, request, &response, 10*time.Second)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
	}

	if response.Method != "response" {
		//TODO(CC): error
	}
	return response.Data
}

func (c *Client) requestMediaNoParams(method string) jsonMap {
	params := map[string]interface{}{}
	return c.requestMedia(method, params)
}

// WebSocket
//-------------------

func (c *Client) readPump() {
	defer func() {
		c.sessionSub.Unsubscribe()
		c.selfSub.Unsubscribe()

		c.clientGroup.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			} else {
				fmt.Println(err)
				fmt.Println("websocket close")
			}
			break
		}
		c.recv <- message
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		//send message to client
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}

		case message, ok := <-c.recv: //TODO: move this case to a single select
			if !ok {
				//TODO(CC):
			}
			c.handleMessage(message)
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

type RequestMessage struct {
	Id     int    `json:"id"`
	Method string `json:"method"`
	Params struct {
		Event string                 `json:"event"`
		Data  map[string]interface{} `json:"data"`
	} `json:"params"`
}

func newClient(clientGroup *ClientGroup, conn *websocket.Conn, tokenId string, sessionId string, metadata map[string]string) *Client {
	fmt.Println("create client")
	client := &Client{clientGroup: clientGroup, tokenId: tokenId, sessionId: sessionId, metadata: metadata, isPub: false, isSub: false}
	client.send = make(chan []byte)
	client.recv = make(chan []byte)
	client.conn = conn
	return client
}
