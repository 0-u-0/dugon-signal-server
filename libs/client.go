package libs

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 10240 //TODO: adjust
)

type jsonMap = map[string]interface{}

type client struct {
	clientGroup *ClientGroup
	userId      string
	tokenId     string
	roomId      string
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
	RoomId   string            `json:"roomId"`
	UserId   string            `json:"userId"`
	TokenId  string            `json:"tokenId"`
	Metadata map[string]string `json:"metadata"`
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
		Log.Errorf("Client message json parse error : %v", jsonErr)
		return
	}

	//fmt.Println(len(message))
	Log.Tracef("message : %v", requestMes)
	if requestMes.Method == "request" {
		data := requestMes.Params.Data
		switch requestMes.Params.Event {
		case "join":
			pub := requestMes.Params.Data["pub"].(bool)
			sub := requestMes.Params.Data["sub"].(bool)

			if mediaIdAny, ok := requestMes.Params.Data["mediaId"]; ok {
				mediaId := fmt.Sprintf("%v", mediaIdAny)
				c.selectMediaServer(mediaId)
			} else {
				c.selectMediaServer("")
			}

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
				"publisherId": senderData["senderId"],
			})

			c.publish2Session("publish", jsonMap{
				"mediaId":     c.mediaServer.Id,
				"area":        c.mediaServer.Area,
				"host":        c.mediaServer.Host,
				"transportId": c.pubTransId,
				"publisherId": senderData["senderId"],
				"metadata":    requestMes.Params.Data["metadata"],
			})
		case "unpublish":
			c.requestMedia("unpublish", jsonMap{
				"transportId": data["transportId"],
				"senderId":    data["publisherId"],
			})
			c.responseClientWithoutData(requestMes.Id)

			c.publish2Session("unpublish", jsonMap{
				"publisherId": data["publisherId"],
			})
		case "subscribe":

			subData := c.requestMedia("subscribe", jsonMap{
				"mediaId":           data["mediaId"],
				"remoteTransportId": data["transportId"],
				"transportId":       c.subTransId,
				"senderId":          data["publisherId"],
			})

			c.responseClient(requestMes.Id, jsonMap{
				"codec":       subData["codec"],
				"receiverId":  subData["receiverId"],
				"publisherId": data["publisherId"],
			})
		case "unsubscribe":
			c.requestMedia("unsubscribe", jsonMap{
				"transportId": data["transportId"],
				"senderId":    data["publisherId"],
			})
			c.responseClientWithoutData(requestMes.Id)
		case "pause":
			c.requestMedia("pause", jsonMap{
				"transportId": data["transportId"],
				"senderId":    data["publisherId"],
				"role":        data["role"],
			})
			c.responseClientWithoutData(requestMes.Id)
			if data["role"].(string) == "pub" {
				c.publish2Session("pause", jsonMap{
					"publisherId": data["publisherId"],
				})
			}
		case "resume":
			c.requestMedia("resume", jsonMap{
				"transportId": data["transportId"],
				"senderId":    data["publisherId"],
				"role":        data["role"],
			})
			c.responseClientWithoutData(requestMes.Id)
			if data["role"].(string) == "pub" {
				c.publish2Session("resume", jsonMap{
					"publisherId": data["publisherId"],
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

// NATS
// ---------------------
func (c *client) publish2Session(method string, data jsonMap) {
	sessionSubject := fmt.Sprintf("signal.%s.@", c.sessionId)

	c.clientGroup.nc.Publish(sessionSubject, jsonMap{
		"userId": c.userId,
		"method": method,
		"data":   data,
	})
}

func (c *client) publish2One(userId string, method string, data jsonMap) {
	oneSubject := fmt.Sprintf("signal.%s.%s", c.sessionId, userId)

	c.clientGroup.nc.Publish(oneSubject, jsonMap{
		"userId": c.userId,
		"method": method,
		"data":   data,
	})
}

type natsSubscribedMessage struct {
	UserId string  `json:"userId"`
	Method string  `json:"method"`
	Data   jsonMap `json:"data"`
}

func (c *client) notifySenders(userId string) {

	sendersData := c.requestMedia("senders", jsonMap{
		"transportId": c.pubTransId,
	})
	senders := sendersData["senders"].([]interface{})

	for _, s := range senders {
		sender := s.(jsonMap)
		c.publish2One(userId, "publish", jsonMap{
			"mediaId":     c.mediaServer.Id,
			"area":        c.mediaServer.Area,
			"host":        c.mediaServer.Host,
			"transportId": c.pubTransId,
			"publisherId": sender["id"],
			"metadata":    sender["metadata"],
		})
	}
}

// func (c *client) notifySender2Client(userId string, senderId string, metadata interface{}) {

// 	subData := c.requestMedia("subscribe", jsonMap{
// 		"transportId": c.subTransId,
// 		"senderId":    senderId,
// 	})

// 	c.notification("publish", jsonMap{
// 		"codec":      subData["codec"],
// 		"receiverId": subData["receiverId"],
// 		"senderId":   senderId,
// 		"userId":     userId,
// 		"metadata":   metadata,
// 	})

// }

func (c *client) subscribeNATS() {
	selfSubject := fmt.Sprintf("signal.%s.%s", c.sessionId, c.userId)
	//TODO: error
	selfSub, _ := c.clientGroup.nc.Subscribe(selfSubject, func(m *nats.Msg) {
		Log.Tracef("Self NATS received a message: %s \n", string(m.Data))

		var msg natsSubscribedMessage
		err := json.Unmarshal(m.Data, &msg)
		if err != nil {
			Log.Warnf("Self NATS json decode error : %v+\n", err)
		}

		userId := msg.UserId
		switch msg.Method {
		case "join":
			metadata := msg.Data["metadata"]
			sub := msg.Data["sub"].(bool)

			c.notification("join", jsonMap{
				"userId":   userId,
				"metadata": metadata,
			})

			//FIXME: maybe useless
			if c.isPub && sub {
				c.notifySenders(userId)
			}
		case "publish":
			c.notification("publish", jsonMap{
				"mediaId":     msg.Data["mediaId"],
				"area":        msg.Data["area"],
				"host":        msg.Data["host"],
				"transportId": msg.Data["transportId"],
				"publisherId": msg.Data["publisherId"],
				"metadata":    msg.Data["metadata"],
				"userId":      userId,
			})
			//c.notifySender2Client(tokenId, senderId, metadata)

		}

	})
	c.selfSub = selfSub

	sessionSubject := fmt.Sprintf("signal.%s.@", c.sessionId)
	//TODO(CC): error
	sessionSub, _ := c.clientGroup.nc.Subscribe(sessionSubject, func(m *nats.Msg) {
		Log.Tracef("Session NATS received a message: %s \n", string(m.Data))

		var msg natsSubscribedMessage
		err := json.Unmarshal(m.Data, &msg)
		if err != nil {
			Log.Warnf("Session NATS json decode error : %v\n", err)
		}

		userId := msg.UserId
		if userId != c.userId {
			switch msg.Method {
			case "join":
				metadata := msg.Data["metadata"]
				sub := msg.Data["sub"].(bool)

				c.notification("join", jsonMap{
					"userId":   userId,
					"metadata": metadata,
				})

				c.publish2One(userId, "join", jsonMap{
					"metadata": c.metadata,
					"pub":      c.isPub,
					"sub":      c.isSub,
				})

				if c.isPub && sub {
					c.notifySenders(userId)
				}

			case "leave":
				c.notification("leave", jsonMap{
					"userId": userId,
				})
			case "publish":
				c.notification("publish", jsonMap{
					"mediaId":     msg.Data["mediaId"],
					"area":        msg.Data["area"],
					"host":        msg.Data["host"],
					"transportId": msg.Data["transportId"],
					"publisherId": msg.Data["publisherId"],
					"metadata":    msg.Data["metadata"],
					"userId":      userId,
				})
				//c.notifySender2Client(tokenId, senderId, metadata)
			case "unpublish":
				c.notification("unpublish", jsonMap{
					"publisherId": msg.Data["publisherId"],
					"userId":      userId,
				})
			case "pause":
				c.notification("pause", jsonMap{
					"publisherId": msg.Data["publisherId"],
				})
			case "resume":
				c.notification("resume", jsonMap{
					"publisherId": msg.Data["publisherId"],
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
				Log.Warnf("Websocket error: %v", err)
			} else {
				Log.Debugf("Websocket closed, error : %v", err)

				c.selfSub.Unsubscribe()
				c.sessionSub.Unsubscribe()

				c.publish2Session("leave", jsonMap{})

				if c.isPub {
					c.requestMedia("close", jsonMap{
						"transportId": c.pubTransId,
						"role":        "pub",
					})
				}

				if c.isSub {
					c.requestMedia("close", jsonMap{
						"transportId": c.subTransId,
						"role":        "sub",
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
				Log.Warnf("Websocket json send error : %v", err)
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

// TODO(CC): add exist
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

func newClient(clientGroup *ClientGroup, conn *websocket.Conn, parameters requestParams) *client {
	// tokenId string, sessionId string, metadata map[string]string
	Log.Infof("create client %s, %s", parameters.RoomId, parameters.UserId)
	client := &client{clientGroup: clientGroup,
		userId: parameters.UserId, tokenId: parameters.TokenId,
		roomId: parameters.RoomId, sessionId: parameters.RoomId,
		metadata: parameters.Metadata,
		isPub:    false, isSub: false}
	client.send = make(chan interface{})
	client.recv = make(chan []byte)
	client.conn = conn
	return client
}
