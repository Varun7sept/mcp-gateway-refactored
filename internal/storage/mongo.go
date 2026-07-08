package storage

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/varunbanda/mcp-gateway/internal/common"
)

type ChatStore struct {
	sessions *mongo.Collection
	messages *mongo.Collection
}

func NewChatStore(db *mongo.Database) *ChatStore {
	s := &ChatStore{
		sessions: db.Collection("chat_sessions"),
		messages: db.Collection("chat_messages"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := s.sessions.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "username", Value: 1}, {Key: "updated_at", Value: -1}},
	}); err != nil {
		log.Printf("WARNING: failed to create session index: %v", err)
	}
	if _, err := s.messages.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "session_id", Value: 1}, {Key: "created_at", Value: 1}},
	}); err != nil {
		log.Printf("WARNING: failed to create message index: %v", err)
	}
	return s
}

func (cs *ChatStore) CreateSession(ctx context.Context, username, title string) (*common.ChatSession, error) {
	now := time.Now()
	oid := primitive.NewObjectID()
	_, err := cs.sessions.InsertOne(ctx, bson.M{
		"_id": oid, "username": username, "title": title,
		"created_at": now, "updated_at": now,
	})
	if err != nil {
		return nil, err
	}
	return &common.ChatSession{ID: oid.Hex(), Username: username, Title: title, CreatedAt: now, UpdatedAt: now}, nil
}

func (cs *ChatStore) ListSessions(ctx context.Context, username string) ([]common.ChatSession, error) {
	cursor, err := cs.sessions.Find(ctx, bson.M{"username": username},
		options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var raw []bson.M
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, err
	}
	sessions := make([]common.ChatSession, 0, len(raw))
	for _, r := range raw {
		sessions = append(sessions, bsonToSession(r))
	}
	return sessions, nil
}

func (cs *ChatStore) GetSession(ctx context.Context, id, username string) (*common.ChatSession, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var raw bson.M
	if err := cs.sessions.FindOne(ctx, bson.M{"_id": oid, "username": username}).Decode(&raw); err != nil {
		return nil, err
	}
	s := bsonToSession(raw)
	return &s, nil
}

func (cs *ChatStore) DeleteSession(ctx context.Context, id, username string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	if _, err := cs.sessions.DeleteOne(ctx, bson.M{"_id": oid, "username": username}); err != nil {
		return err
	}
	cs.messages.DeleteMany(ctx, bson.M{"session_id": oid.Hex()})
	return nil
}

func (cs *ChatStore) UpdateSessionTitle(ctx context.Context, sessionID, username, title string) error {
	oid, err := primitive.ObjectIDFromHex(sessionID)
	if err != nil {
		return err
	}
	_, err = cs.sessions.UpdateOne(ctx,
		bson.M{"_id": oid, "username": username},
		bson.M{"$set": bson.M{"title": title, "updated_at": time.Now()}})
	return err
}

func (cs *ChatStore) AddMessage(ctx context.Context, sessionID, role, content string, meta map[string]any) error {
	oid, err := primitive.ObjectIDFromHex(sessionID)
	if err != nil {
		return err
	}
	_, err = cs.messages.InsertOne(ctx, bson.M{
		"session_id": oid.Hex(), "role": role, "content": content, "meta": meta, "created_at": time.Now(),
	})
	if err != nil {
		return err
	}
	cs.sessions.UpdateByID(ctx, oid, bson.M{"$set": bson.M{"updated_at": time.Now()}})
	return nil
}

func (cs *ChatStore) GetMessages(ctx context.Context, sessionID string) ([]common.ChatMessage, error) {
	cursor, err := cs.messages.Find(ctx, bson.M{"session_id": sessionID},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var raw []bson.M
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, err
	}
	msgs := make([]common.ChatMessage, 0, len(raw))
	for _, r := range raw {
		msgs = append(msgs, bsonToMessage(r))
	}
	return msgs, nil
}

func (cs *ChatStore) GetRecentMessages(ctx context.Context, sessionID string, limit int) ([]common.ChatMessage, error) {
	cursor, err := cs.messages.Find(ctx, bson.M{"session_id": sessionID},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var raw []bson.M
	cursor.All(ctx, &raw)
	msgs := make([]common.ChatMessage, 0, len(raw))
	for i := len(raw) - 1; i >= 0; i-- {
		msgs = append(msgs, bsonToMessage(raw[i]))
	}
	return msgs, nil
}

func bsonToSession(r bson.M) common.ChatSession {
	s := common.ChatSession{
		Username: getStr(r, "username"), Title: getStr(r, "title"),
		CreatedAt: getTime(r, "created_at"), UpdatedAt: getTime(r, "updated_at"),
	}
	if id, ok := r["_id"].(primitive.ObjectID); ok {
		s.ID = id.Hex()
	}
	return s
}

func bsonToMessage(r bson.M) common.ChatMessage {
	m := common.ChatMessage{
		Role: getStr(r, "role"), Content: getStr(r, "content"),
		CreatedAt: getTime(r, "created_at"),
	}
	if meta, ok := r["meta"]; ok {
		if mm, ok := meta.(map[string]any); ok {
			m.Meta = mm
		} else if mm, ok := meta.(bson.M); ok {
			m.Meta = mm
		}
	}
	return m
}

func getStr(m bson.M, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getTime(m bson.M, key string) time.Time {
	if v, ok := m[key]; ok {
		if t, ok := v.(time.Time); ok {
			return t
		}
	}
	return time.Time{}
}
