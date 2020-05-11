package libs

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
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

type client struct {
	clientGroup *ClientGroup
	tokenId     string
	sessionId   string
	metadata    map[string]string
	isPub       bool
	isSub       bool
	pubTransId  string
	subTransId  string
	conn        *websocket.Conn
	send        chan interface{}
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
//func (c *Client) idGenerator(role string) string {
//	data := []byte(fmt.Sprintf("%s@%s@%s", c.sessionId, c.tokenId, role))
//	has := md5.Sum(data)
//	return fmt.Sprintf("%x", has)
//}

//func (c *Client) sendJson(v interface{}) {
//	if err := c.conn.WriteJSON(v); err != nil {
//		fmt.Println(err)
//		//TODO:
//	}
//}

func (c *client) responseClient(id int, params interface{}) {
	response := jsonMap{
		"method": "response",
		"id":     id,
		"params": params,
	}
	c.send <- response
	//c.sendJson(response)
}

func (c *client) notification(event string, data interface{}) {
	response := jsonMap{
		"method": "notification",
		"params": jsonMap{
			"event": event,
			"data":  data,
		},
	}
	//c.sendJson(response)
	c.send <- response
}

func (c *client) responseClientWithoutData(id int) {
	c.responseClient(id, map[string]string{})
}

func (c *client) selectMediaServer(mediaId string) {

	selectedMedia := mediaId
	if len(c.clientGroup.mediaServers) > 0 {
		if c.mediaServer == nil {
			ms, ok := c.clientGroup.mediaServers[selectedMedia]
			if ok {
				c.mediaServer = ms
			} else {
				rand.Seed(time.Now().Unix())
				keys := reflect.ValueOf(c.clientGroup.mediaServers).MapKeys()
				selectedMedia = keys[rand.Intn(len(keys))].String()
				//TODO(CC): check
				ms, _ := c.clientGroup.mediaServers[selectedMedia]
				c.mediaServer = ms
			}
		}
	} else {
		//TODO(CC): no media server
	}
}

func (c *client) handleClientMessage(message []byte) {
	var requestMes *requestMessage
	jsonErr := json.Unmarshal(message, &requestMes)
	if jsonErr != nil {
		//TODO(CC):
		return
	}

	//fmt.Println(len(message))
	if requestMes.Method == "request" {
		data := requestMes.Params.Data
		switch requestMes.Params.Event {
		case "join":
			pub := requestMes.Params.Data["pub"].(bool)
			sub := requestMes.Params.Data["sub"].(bool)
			mediaId := requestMes.Params.Data["mediaId"].(string)
			//FIXME: ...
			c.selectMediaServer(mediaId)

			codecData := c.requestMediaNoParams("codecs")

			responseParams := jsonMap{
				"codecs": codecData["codecs"],
			}

			if pub {
				c.isPub = true

				transportId, _ := uuid.NewUUID()
				c.pubTransId = transportId.String()

				mediaRequest := jsonMap{
					"transportId": c.pubTransId,
					"role":        "pub",
				}
				pubData := c.requestMedia("transport", mediaRequest)
				responseParams["pub"] = pubData["transportParameters"]
			}

			if sub {
				c.isSub = true

				transportId, _ := uuid.NewUUID()
				c.subTransId = transportId.String()

				mediaRequest := jsonMap{
					"transportId": c.subTransId,
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
				"mediaId":     c.mediaServer.Id,
				"area":        c.mediaServer.Area,
				"host":        c.mediaServer.Host,
				"transportId": c.pubTransId,
				"senderId":    senderData["senderId"],
				"metadata":    requestMes.Params.Data["metadata"],
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
		case "subscribe":

			subData := c.requestMedia("subscribe", jsonMap{
				"mediaId":           data["mediaId"],
				"remoteTransportId": data["transportId"],
				"transportId":       c.subTransId,
				"senderId":          data["senderId"],
			})

			c.responseClient(requestMes.Id, jsonMap{
				"codec":      subData["codec"],
				"receiverId": subData["receiverId"],
				"senderId":   data["senderId"],
			})
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
	Method string  `json:"method"`
	Params jsonMap `json:"params"`
}

type mediaResponse struct {
	Method string  `json:"method"`
	Data   jsonMap `json:"data"`
}

//NATS
//---------------------
func (c *client) publish2Session(method string, data jsonMap) {
	sessionSubject := fmt.Sprintf("signal.%s.@", c.sessionId)

	c.clientGroup.nc.Publish(sessionSubject, jsonMap{
		"tokenId": c.tokenId,
		"method":  method,
		"data":    data,
	})
}

func (c *client) publish2One(tokenId string, method string, data jsonMap) {
	oneSubject := fmt.Sprintf("signal.%s.%s", c.sessionId, tokenId)

	c.clientGroup.nc.Publish(oneSubject, jsonMap{
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

func (c *client) notifySenders(tokenId string) {

	sendersData := c.requestMedia("senders", jsonMap{
		"transportId": c.pubTransId,
	})
	senders := sendersData["senders"].([]interface{})

	for _, s := range senders {
		sender := s.(jsonMap)
		c.publish2One(tokenId, "publish", jsonMap{
			"mediaId":     c.mediaServer.Id,
			"area":        c.mediaServer.Area,
			"host":        c.mediaServer.Host,
			"transportId": c.pubTransId,
			"senderId":    sender["id"],
			"metadata":    sender["metadata"],
		})
	}
}

func (c *client) notifySender2Client(tokenId string, senderId string, metadata interface{}) {

	subData := c.requestMedia("subscribe", jsonMap{
		"transportId": c.subTransId,
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

func (c *client) subscribeNATS() {
	selfSubject := fmt.Sprintf("signal.%s.%s", c.sessionId, c.tokenId)
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

			//FIXME: maybe useless
			if c.isPub && sub {
				c.notifySenders(tokenId)
			}
		case "publish":
			c.notification("publish", jsonMap{
				"mediaId":     msg.Data["mediaId"],
				"area":        msg.Data["area"],
				"host":        msg.Data["host"],
				"transportId": msg.Data["transportId"],
				"senderId":    msg.Data["senderId"],
				"metadata":    msg.Data["metadata"],
				"tokenId":     tokenId,
			})
			//c.notifySender2Client(tokenId, senderId, metadata)

		}

	})
	c.selfSub = selfSub

	sessionSubject := fmt.Sprintf("signal.%s.@", c.sessionId)
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
				c.notification("publish", jsonMap{
					"mediaId":     msg.Data["mediaId"],
					"area":        msg.Data["area"],
					"host":        msg.Data["host"],
					"transportId": msg.Data["transportId"],
					"senderId":    msg.Data["senderId"],
					"metadata":    msg.Data["metadata"],
					"tokenId":     tokenId,
				})
				//c.notifySender2Client(tokenId, senderId, metadata)
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
		}

	})
	c.sessionSub = sessionSub
}

func (c *client) requestMedia(method string, params jsonMap) jsonMap {

	request := MediaRequest{Method: method, Params: params}

	var response mediaResponse

	mediaSubject := fmt.Sprintf("media.%s", c.mediaServer.Id)
	err := c.clientGroup.nc.Request(mediaSubject, request, &response, 10*time.Second)
	if err != nil {
		Log.Warnf("Request failed: %s %v\n", method, err)
	}

	if response.Method != "response" {
		//TODO(CC): error
	}
	return response.Data
}

//func (c *client) notifyMedia(method string, params jsonMap)  {
//
//	request := MediaRequest{Method: method, Params: params}
//
//	mediaSubject := "media.@"
//	err := c.clientGroup.nc.Publish(mediaSubject, request)
//	if err != nil {
//		Log.Warnf("Notify media failed: %s %v\n", method, err)
//	}
//}
//

func (c *client) requestMediaNoParams(method string) jsonMap {
	params := jsonMap{}
	return c.requestMedia(method, params)
}

// WebSocket
//-------------------

func (c *client) readPump() {
	defer func() {

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

				c.selfSub.Unsubscribe()
				c.sessionSub.Unsubscribe()

				c.publish2Session("leave", jsonMap{})

				if c.isPub {
					c.requestMedia("close", jsonMap{
						"transportId": c.pubTransId,
					})
				}

				if c.isSub {
					c.requestMedia("close", jsonMap{
						"transportId": c.subTransId,
					})
				}
			}
			break
		}
		c.recv <- message
	}
}

func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		//send message to client
		case jsonMsg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(jsonMsg); err != nil {
				fmt.Println(err)
				//TODO:
			}
			//w, err := c.conn.NextWriter(websocket.TextMessage)
			//if err != nil {
			//	return
			//}
			//w.Write(message)
			//
			//if err := w.Close(); err != nil {
			//	return
			//}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

//TODO(CC): add exist
func (c *client) processPump() {
	for {
		select {
		case message, ok := <-c.recv: //TODO: move this case to a single select
			if !ok {
				//TODO(CC):
			}
			c.handleClientMessage(message)
		}
	}
}

type requestMessage struct {
	Id     int    `json:"id"`
	Method string `json:"method"`
	Params struct {
		Event string  `json:"event"`
		Data  jsonMap `json:"data"`
	} `json:"params"`
}

func newClient(clientGroup *ClientGroup, conn *websocket.Conn, tokenId string, sessionId string, metadata map[string]string) *client {
	fmt.Println("create client")
	client := &client{clientGroup: clientGroup, tokenId: tokenId, sessionId: sessionId, metadata: metadata, isPub: false, isSub: false}
	client.send = make(chan interface{})
	client.recv = make(chan []byte)
	client.conn = conn
	return client
}
