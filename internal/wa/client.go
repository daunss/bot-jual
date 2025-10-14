package wa

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"bot-jual/internal/metrics"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"
)

// Config holds configuration to initialise the WhatsApp client.
type Config struct {
	StorePath string
	LogLevel  string
	Metrics   *metrics.Metrics
}

// Client wraps the WhatsMeow client and associated dependencies.
type Client struct {
	client    *whatsmeow.Client
	logger    *slog.Logger
	metrics   *metrics.Metrics
	processor MessageProcessor
}

// MessageProcessor handles inbound WhatsApp messages.
type MessageProcessor interface {
	ProcessMessage(ctx context.Context, evt *events.Message)
}

type replyContextKey struct{}

// ReplyMetadata carries information for quoting a previous message.
type ReplyMetadata struct {
	Message *waProto.Message
	Info    types.MessageInfo
}

// WithReply attaches reply metadata to the context so outgoing messages quote the given event.
func WithReply(ctx context.Context, evt *events.Message) context.Context {
	if evt == nil || evt.Message == nil {
		return ctx
	}
	cloned, ok := proto.Clone(evt.Message).(*waProto.Message)
	if !ok {
		cloned = evt.Message
	}
	meta := &ReplyMetadata{
		Message: cloned,
		Info:    evt.Info,
	}
	return context.WithValue(ctx, replyContextKey{}, meta)
}

func replyFromContext(ctx context.Context) *ReplyMetadata {
	if ctx == nil {
		return nil
	}
	meta, _ := ctx.Value(replyContextKey{}).(*ReplyMetadata)
	return meta
}

// New creates a new WhatsApp client instance backed by an SQLite store.
func New(ctx context.Context, cfg Config, logger *slog.Logger) (*Client, error) {
	if cfg.StorePath == "" {
		return nil, errors.New("store path is required")
	}

	if err := ensureDir(filepath.Dir(cfg.StorePath)); err != nil {
		return nil, fmt.Errorf("ensure store dir: %w", err)
	}

	storeLogger := waLog.Stdout("whatsmeow/sqlstore", cfg.LogLevel, true)
	container, err := sqlstore.New(ctx, "sqlite", fmt.Sprintf("file:%s?_pragma=busy_timeout=10000&_pragma=foreign_keys(ON)", cfg.StorePath), storeLogger)
	if err != nil {
		return nil, fmt.Errorf("create sqlstore: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("get device: %w", err)
	}

	waLogger := waLog.Stdout("whatsmeow/client", cfg.LogLevel, true)
	client := whatsmeow.NewClient(deviceStore, waLogger)

	wc := &Client{
		client:  client,
		logger:  logger.With("component", "wa"),
		metrics: cfg.Metrics,
	}
	client.AddEventHandler(wc.handleEvent)

	return wc, nil
}

// Start connects the client and handles login/QR pairing flow.
func (c *Client) Start(ctx context.Context) error {
	if c.client.Store.ID == nil {
		c.logger.Info("pairing required, waiting for QR scan")
		qrChan, err := c.client.GetQRChannel(ctx)
		if err != nil {
			return fmt.Errorf("get qr channel: %w", err)
		}

		go func() {
			for evt := range qrChan {
				if evt.Event == "code" {
					c.logger.Info("scan the QR code with WhatsApp", "qr", evt.Code)
				} else {
					c.logger.Info("pairing event received", "event", evt.Event)
				}
			}
		}()
	}

	if err := c.client.Connect(); err != nil {
		return fmt.Errorf("connect wa client: %w", err)
	}

	c.logger.Info("whatsapp client connected")
	return nil
}

// Close disconnects the WhatsApp client.
func (c *Client) Close() {
	if c.client != nil {
		c.client.Disconnect()
	}
}

func (c *Client) handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		c.handleMessage(v)
	case *events.Connected:
		c.logger.Info("device connected")
	case *events.Disconnected:
		c.logger.Warn("device disconnected")
	}
}

func (c *Client) handleMessage(evt *events.Message) {
	msg := evt.Message
	if msg == nil {
		return
	}

	sender := evt.Info.Sender.String()

	switch {
	case msg.GetConversation() != "":
		c.logger.Info("received text message", "from", sender, "text", msg.GetConversation())
	case msg.ExtendedTextMessage != nil:
		c.logger.Info("received extended text message", "from", sender, "text", msg.GetExtendedTextMessage().GetText())
	case msg.ImageMessage != nil:
		c.logger.Info("received image message", "from", sender, "caption", msg.GetImageMessage().GetCaption())
	case msg.VideoMessage != nil:
		c.logger.Info("received video message", "from", sender, "caption", msg.GetVideoMessage().GetCaption())
	case msg.AudioMessage != nil:
		c.logger.Info("received audio message", "from", sender, "ptt", msg.GetAudioMessage().GetPTT())
	default:
		c.logger.Info("received unsupported message type", "from", sender)
	}

	if c.processor != nil {
		go c.processor.ProcessMessage(context.Background(), evt)
	}
}

func ensureDir(dir string) error {
	if dir == "." || dir == "" {
		return nil
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0o755)
	}
	return nil
}

// SetMessageProcessor registers message processor callback.
func (c *Client) SetMessageProcessor(processor MessageProcessor) {
	c.processor = processor
}

// SendText sends a text message to the specified JID.
func (c *Client) SendText(ctx context.Context, to types.JID, text string) error {
	reply := replyFromContext(ctx)
	var message *waProto.Message
	if reply != nil && reply.Message != nil {
		contextInfo := &waProto.ContextInfo{
			StanzaID:      proto.String(string(reply.Info.ID)),
			Participant:   proto.String(reply.Info.Sender.ToNonAD().String()),
			RemoteJID:     proto.String(reply.Info.Chat.String()),
			QuotedMessage: reply.Message,
			QuotedType:    waProto.ContextInfo_EXPLICIT.Enum(),
		}
		message = &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        proto.String(text),
				ContextInfo: contextInfo,
			},
		}
	} else {
		message = &waProto.Message{
			Conversation: proto.String(text),
		}
	}
	_, err := c.client.SendMessage(ctx, to, message)
	if err != nil {
		return fmt.Errorf("send text: %w", err)
	}
	if c.metrics != nil {
		c.metrics.WAOutgoingMessages.WithLabelValues("text").Inc()
	}
	return nil
}

// SendImage uploads and sends an image message to the specified JID.
func (c *Client) SendImage(ctx context.Context, to types.JID, data []byte, mimeType, caption string) error {
	if len(data) == 0 {
		return errors.New("send image: empty data")
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
		if mimeType == "" {
			mimeType = "image/png"
		}
	}
	uploadResp, err := c.client.Upload(ctx, data, whatsmeow.MediaImage)
	if err != nil {
		return fmt.Errorf("upload image: %w", err)
	}

	imageMsg := &waProto.ImageMessage{
		URL:           proto.String(uploadResp.URL),
		DirectPath:    proto.String(uploadResp.DirectPath),
		MediaKey:      uploadResp.MediaKey,
		FileEncSHA256: uploadResp.FileEncSHA256,
		FileSHA256:    uploadResp.FileSHA256,
		FileLength:    proto.Uint64(uploadResp.FileLength),
		Mimetype:      proto.String(mimeType),
	}
	if caption != "" {
		imageMsg.Caption = proto.String(caption)
	}

	message := &waProto.Message{
		ImageMessage: imageMsg,
	}
	if _, err := c.client.SendMessage(ctx, to, message); err != nil {
		return fmt.Errorf("send image: %w", err)
	}
	if c.metrics != nil {
		c.metrics.WAOutgoingMessages.WithLabelValues("image").Inc()
	}
	return nil
}

// DownloadMedia downloads the media content from a message and returns bytes and mime type.
func (c *Client) DownloadMedia(ctx context.Context, msg *waProto.Message) ([]byte, string, error) {
	data, err := c.client.DownloadAny(ctx, msg)
	if err != nil {
		return nil, "", fmt.Errorf("download media: %w", err)
	}

	mime := "application/octet-stream"
	switch {
	case msg.ImageMessage != nil:
		if m := msg.ImageMessage.GetMimetype(); m != "" {
			mime = m
		}
	case msg.VideoMessage != nil:
		if m := msg.VideoMessage.GetMimetype(); m != "" {
			mime = m
		}
	case msg.AudioMessage != nil:
		if m := msg.AudioMessage.GetMimetype(); m != "" {
			mime = m
		}
	case msg.DocumentMessage != nil:
		if m := msg.DocumentMessage.GetMimetype(); m != "" {
			mime = m
		}
	}
	return data, mime, nil
}
