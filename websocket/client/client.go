// Package client implements one native QQ WebSocket connection per shard.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/errs"
	"github.com/WindowsSov8forUs/botgo-plus/event"
	"github.com/WindowsSov8forUs/botgo-plus/log"
	"github.com/WindowsSov8forUs/botgo-plus/token"
	"github.com/WindowsSov8forUs/botgo-plus/websocket"
	wss "github.com/gorilla/websocket"
)

// DefaultQueueSize retains the upstream queue capacity. A full queue applies backpressure.
const DefaultQueueSize = 10000
const maxFrameBytes int64 = 4 * 1024 * 1024

type messageChan chan *dto.WSPayload
type closeErrorChan chan error

type Client struct {
	stateMu          sync.RWMutex
	session          *dto.Session
	version          int
	user             *dto.WSUser
	connMu           sync.RWMutex
	conn             *wss.Conn
	writeMu          sync.Mutex
	messageQueue     messageChan
	closeChan        closeErrorChan
	heartBeatTicker  *time.Ticker
	heartbeatPending atomic.Bool
	receivedSeq      atomic.Uint32
	lastToken        atomic.Value
	closed           atomic.Bool
	listening        atomic.Bool
	closeOnce        sync.Once
	ctx              context.Context
	cancel           context.CancelFunc
}

func Setup() { websocket.Register(&Client{}) }

func (c *Client) New(session dto.Session) websocket.WebSocket {
	if session.AppID == "" {
		if source, ok := session.TokenSource.(interface{ GetAppID() string }); ok {
			session.AppID = source.GetAppID()
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	if session.Shards.ShardCount == 0 {
		session.Shards.ShardCount = 1
	}
	if session.Intent == 0 {
		session.Intent = dto.IntentGuilds
	}
	result := &Client{session: &session, messageQueue: make(messageChan, DefaultQueueSize), closeChan: make(closeErrorChan, 2), heartBeatTicker: time.NewTicker(60 * time.Second), ctx: ctx, cancel: cancel}
	result.receivedSeq.Store(session.LastSeq)
	return result
}

func (c *Client) connection() *wss.Conn { c.connMu.RLock(); defer c.connMu.RUnlock(); return c.conn }

// Session returns a snapshot. Mutating it never changes an active connection.
func (c *Client) Session() *dto.Session {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	if c.session == nil {
		return &dto.Session{}
	}
	copy := *c.session
	return &copy
}

func (c *Client) Connect() error {
	if c.closed.Load() || c.ctx == nil {
		return errors.New("create a new QQ connection before Connect")
	}
	session := c.Session()
	u, err := url.Parse(session.URL)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "ws" && u.Scheme != "wss") {
		return errs.ErrURLInvalid
	}
	if session.Shards.ShardID >= session.Shards.ShardCount {
		return errors.New("invalid shard configuration")
	}
	dialer := *wss.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second
	conn, response, err := dialer.DialContext(c.ctx, session.URL, nil)
	if response != nil && response.Body != nil && err != nil {
		response.Body.Close()
	}
	if err != nil {
		return err
	}
	conn.SetReadLimit(maxFrameBytes)
	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	c.connMu.Lock()
	if c.closed.Load() || c.conn != nil {
		c.connMu.Unlock()
		conn.Close()
		return errors.New("QQ connection is closed or already connected")
	}
	c.conn = conn
	c.connMu.Unlock()
	log.Infof("%s connected", session)
	return nil
}

func (c *Client) notify(err error) {
	select {
	case c.closeChan <- err:
	default:
	}
}

func (c *Client) Listening() error {
	if c.connection() == nil || c.closed.Load() {
		return errors.New("QQ connection is not open")
	}
	if !c.listening.CompareAndSwap(false, true) {
		return errors.New("Listening may only be called once")
	}
	defer c.Close()
	go c.readMessageToQueue()
	go c.listenMessageAndHandle()
	resumeSignal := make(chan os.Signal, 1)
	if websocket.ResumeSignal >= syscall.SIGHUP {
		signal.Notify(resumeSignal, websocket.ResumeSignal)
		defer signal.Stop(resumeSignal)
	}
	for {
		select {
		case <-c.ctx.Done():
			return c.ctx.Err()
		case <-resumeSignal:
			return errs.ErrNeedReConnect
		case err := <-c.closeChan:
			err = c.classifyClose(err)
			if event.DefaultHandlers.ErrorNotify != nil {
				event.DefaultHandlers.ErrorNotify(err)
			}
			return err
		case <-c.heartBeatTicker.C:
			if err := c.heartbeatTick(); err != nil {
				return err
			}
		}
	}
}

func (c *Client) classifyClose(err error) error {
	if wss.IsCloseError(err, errs.WSCodeBackendBotOffline, errs.WSCodeBackendBotBanned) {
		return errs.New(errs.CodeConnCloseCantIdentify, "QQ bot is offline or banned")
	}
	if wss.IsCloseError(err, errs.WSCodeBackendAuthenticationFail) {
		if invalidator, ok := c.Session().TokenSource.(interface{ Invalidate(string) bool }); ok {
			if rejected, ok := c.lastToken.Load().(string); ok {
				invalidator.Invalidate(rejected)
			}
		}
		return errs.New(errs.CodeConnCloseCantResume, "QQ gateway rejected authentication")
	}
	var closed *wss.CloseError
	if errors.As(err, &closed) && closed.Code >= 4000 && closed.Code != errs.WSCodeBackendSessionTimeOut {
		return errs.New(errs.CodeConnCloseCantResume, fmt.Sprintf("QQ gateway close code %d", closed.Code))
	}
	return err
}

func (c *Client) heartbeatTick() error {
	if c.heartbeatPending.Swap(true) {
		return errs.ErrNeedReConnect
	}
	return c.Write(&dto.WSPayload{WSPayloadBase: dto.WSPayloadBase{OPCode: dto.WSHeartbeat}, Data: c.receivedSeq.Load()})
}

func (c *Client) Write(message *dto.WSPayload) error {
	if message == nil {
		return errors.New("nil QQ gateway payload")
	}
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	conn := c.connection()
	if conn == nil || c.closed.Load() {
		return errors.New("QQ connection is not open")
	}
	if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		c.notify(err)
		return err
	}
	// Never log IDENTIFY/RESUME tokens or raw event bodies.
	log.Debugf("%s write opcode=%d bytes=%d", c.Session(), message.OPCode, len(data))
	if err := conn.WriteMessage(wss.TextMessage, data); err != nil {
		c.notify(err)
		return err
	}
	return nil
}

func (c *Client) authorization() (string, error) {
	if c.ctx == nil {
		return "", errors.New("create a new QQ connection before authenticating")
	}
	tk, err := token.TokenContext(c.ctx, c.Session().TokenSource)
	if err != nil {
		return "", err
	}
	if tk == nil || tk.AccessToken == "" {
		return "", errors.New("empty QQ gateway token")
	}
	c.lastToken.Store(tk.AccessToken)
	scheme := tk.TokenType
	if scheme == "" {
		scheme = token.TypeQQBot
	}
	return scheme + " " + tk.AccessToken, nil
}

func (c *Client) Resume() error {
	auth, err := c.authorization()
	if err != nil {
		return err
	}
	session := c.Session()
	if session.ID == "" {
		return errors.New("resume requires a QQ session ID")
	}
	return c.Write(&dto.WSPayload{WSPayloadBase: dto.WSPayloadBase{OPCode: dto.WSResume}, Data: &dto.WSResumeData{Token: auth, SessionID: session.ID, Seq: session.LastSeq}})
}
func (c *Client) Identify() error {
	auth, err := c.authorization()
	if err != nil {
		return err
	}
	session := c.Session()
	return c.Write(&dto.WSPayload{WSPayloadBase: dto.WSPayloadBase{OPCode: dto.WSIdentity}, Data: &dto.WSIdentityData{Token: auth, Intents: session.Intent, Shard: []uint32{session.Shards.ShardID, session.Shards.ShardCount}}})
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		// Serialize closure with all session writes so snapshots stay frozen after Close returns.
		c.stateMu.Lock()
		c.closed.Store(true)
		c.stateMu.Unlock()
		if c.cancel != nil {
			c.cancel()
		}
		if conn := c.connection(); conn != nil {
			_ = conn.Close()
		}
		if c.heartBeatTicker != nil {
			c.heartBeatTicker.Stop()
		}
	})
}

func (c *Client) readMessageToQueue() {
	defer close(c.messageQueue)
	conn := c.connection()
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			c.notify(err)
			return
		}
		var envelope struct {
			Op *dto.OPCode `json:"op"`
		}
		if json.Unmarshal(raw, &envelope) != nil || envelope.Op == nil {
			c.notify(errors.New("malformed QQ gateway payload"))
			return
		}
		payload := &dto.WSPayload{}
		if err := json.Unmarshal(raw, payload); err != nil {
			c.notify(err)
			return
		}
		payload.RawMessage = raw
		payload.Session = c.Session()
		if payload.OPCode == dto.WSDispatchEvent {
			c.receivedSeq.Store(payload.Seq)
		}
		if c.isHandleBuildIn(payload) {
			continue
		}
		select {
		case <-c.ctx.Done():
			return
		case c.messageQueue <- payload:
		}
	}
}

func (c *Client) listenMessageAndHandle() {
	defer func() {
		if recover() != nil {
			c.notify(errors.New("QQ event handler panicked"))
		}
	}()
	for {
		if c.ctx.Err() != nil {
			return
		}
		select {
		case <-c.ctx.Done():
			return
		case payload, open := <-c.messageQueue:
			if !open {
				return
			}
			if c.ctx.Err() != nil {
				return
			}
			// Refresh the snapshot at dispatch time: READY may have been queued before this event.
			payload.Session = c.Session()
			if payload.Type == "READY" {
				if err := c.readyHandler(payload); err != nil {
					c.notify(err)
					return
				}
			} else if err := event.ParseAndHandle(payload); err != nil {
				// Preserve upstream behavior: report the application error and continue.
				// Business retries are not implemented by reconnecting the QQ gateway.
				log.Errorf("QQ event handler failed: %v", err)
			}
			c.saveSeq(payload.Seq)
		}
	}
}

func (c *Client) saveSeq(seq uint32) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if !c.closed.Load() && c.session != nil && seq > c.session.LastSeq {
		c.session.LastSeq = seq
	}
}

func (c *Client) isHandleBuildIn(payload *dto.WSPayload) bool {
	switch payload.OPCode {
	case dto.WSHello:
		c.startHeartBeatTicker(payload.RawMessage)
	case dto.WSHeartbeatAck:
		c.heartbeatPending.Store(false)
	case dto.WSHeartbeat:
		c.heartbeatPending.Store(true)
		_ = c.Write(&dto.WSPayload{WSPayloadBase: dto.WSPayloadBase{OPCode: dto.WSHeartbeat}, Data: c.receivedSeq.Load()})
	case dto.WSReconnect:
		c.notify(errs.ErrNeedReConnect)
	case dto.WSInvalidSession:
		c.notify(errs.ErrInvalidSession)
	default:
		return false
	}
	return true
}

func (c *Client) startHeartBeatTicker(raw []byte) {
	var hello dto.WSHelloData
	if err := event.ParseData(raw, &hello); err != nil || hello.HeartbeatInterval <= 0 || hello.HeartbeatInterval > 24*60*60*1000 {
		c.notify(errors.New("invalid QQ heartbeat interval"))
		return
	}
	c.heartBeatTicker.Reset(time.Duration(hello.HeartbeatInterval) * time.Millisecond)
	if conn := c.connection(); conn != nil {
		_ = conn.SetReadDeadline(time.Time{})
	}
}

func (c *Client) readyHandler(payload *dto.WSPayload) error {
	var ready dto.WSReadyData
	if err := event.ParseData(payload.RawMessage, &ready); err != nil {
		return err
	}
	if ready.SessionID == "" || len(ready.Shard) != 2 || ready.Shard[1] == 0 || ready.Shard[0] >= ready.Shard[1] {
		return errors.New("invalid QQ READY payload")
	}
	c.stateMu.Lock()
	if c.closed.Load() {
		c.stateMu.Unlock()
		return context.Canceled
	}
	c.version = ready.Version
	c.session.ID = ready.SessionID
	c.session.Shards = dto.ShardConfig{ShardID: ready.Shard[0], ShardCount: ready.Shard[1]}
	c.user = &dto.WSUser{ID: ready.User.ID, Username: ready.User.Username, Bot: ready.User.Bot}
	c.stateMu.Unlock()
	payload.Session = c.Session()
	if event.DefaultHandlers.Ready != nil {
		event.DefaultHandlers.Ready(payload, &ready)
	}
	return nil
}
