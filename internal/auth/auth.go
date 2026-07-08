// Package auth handles user authentication via JWT tokens stored in MongoDB.
// Responsibilities: signup, login, JWT validation, request logging.
package auth

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Username  string    `bson:"username" json:"username"`
	Email     string    `bson:"email" json:"email"`
	Password  string    `bson:"password" json:"-"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}

type Auth struct {
	users       *mongo.Collection
	requestLogs *mongo.Collection
	jwtSecret   []byte
	db          *mongo.Database
}

func (a *Auth) DB() *mongo.Database {
	return a.db
}

type ctxKey string

const UserKey ctxKey = "username"

func UserFromContext(ctx context.Context) (string, bool) {
	v := ctx.Value(UserKey)
	if v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func New(uri, database string) (*Auth, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable required")
	}

	db := client.Database(database)
	a := &Auth{
		users:       db.Collection("users"),
		requestLogs: db.Collection("request_logs"),
		jwtSecret:   []byte(secret),
	}

	if err := a.ensureIndexes(ctx); err != nil {
		return nil, fmt.Errorf("indexes: %w", err)
	}
	return a, nil
}

func (a *Auth) ensureIndexes(ctx context.Context) error {
	if _, err := a.users.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "username", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)},
	}); err != nil {
		return err
	}
	_, err := a.requestLogs.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "username", Value: 1}, {Key: "created_at", Value: -1}}},
	})
	return err
}

func (a *Auth) Signup(username, email, password string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if len(password) < 6 {
		return "", fmt.Errorf("password must be at least 6 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	user := User{Username: username, Email: email, Password: string(hash), CreatedAt: time.Now()}
	if _, err := a.users.InsertOne(ctx, user); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return "", fmt.Errorf("username or email already exists")
		}
		return "", fmt.Errorf("insert user: %w", err)
	}
	return a.generateToken(username)
}

func (a *Auth) Login(username, password string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var user User
	if err := a.users.FindOne(ctx, bson.M{"username": username}).Decode(&user); err != nil {
		return "", fmt.Errorf("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", fmt.Errorf("invalid credentials")
	}
	return a.generateToken(username)
}

func (a *Auth) ValidateToken(tokenStr string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.jwtSecret, nil
	})
	if err != nil {
		return "", fmt.Errorf("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid token claims")
	}
	username, ok := claims["sub"].(string)
	if !ok {
		return "", fmt.Errorf("invalid token subject")
	}
	return username, nil
}

func (a *Auth) LogRequest(username, method, toolName, serverName, status, errMsg string, latency time.Duration) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		a.requestLogs.InsertOne(ctx, bson.M{
			"username": username, "method": method, "tool_name": toolName,
			"server_name": serverName, "status": status, "error": errMsg,
			"latency_ms": latency.Milliseconds(), "created_at": time.Now(),
		})
	}()
}

func (a *Auth) RecentLogs(n int, username string) []bson.M {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	filter := bson.M{}
	if username != "" {
		filter = bson.M{"username": username}
	}
	cursor, err := a.requestLogs.Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(int64(n)))
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)
	var results []bson.M
	cursor.All(ctx, &results)
	return results
}

func (a *Auth) GetRequestStats(username string) map[string]any {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	match := bson.M{}
	if username != "" {
		match = bson.M{"username": username}
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.M{
			"_id": nil, "total_requests": bson.M{"$sum": 1},
			"success_count": bson.M{"$sum": bson.M{"$cond": []any{bson.M{"$eq": []string{"$status", "success"}}, 1, 0}}},
			"error_count":   bson.M{"$sum": bson.M{"$cond": []any{bson.M{"$eq": []string{"$status", "error"}}, 1, 0}}},
			"avg_latency":   bson.M{"$avg": "$latency_ms"},
		}}},
	}

	stats := map[string]any{
		"total_requests": 0, "success_count": 0, "error_count": 0, "avg_latency_ms": 0,
		"requests_by_tool": map[string]int{}, "requests_by_server": map[string]int{},
	}

	cursor, err := a.requestLogs.Aggregate(ctx, pipeline)
	if err != nil {
		return stats
	}
	defer cursor.Close(ctx)
	var results []bson.M
	cursor.All(ctx, &results)
	if len(results) > 0 {
		r := results[0]
		if v, ok := r["total_requests"]; ok { stats["total_requests"] = v }
		if v, ok := r["success_count"]; ok { stats["success_count"] = v }
		if v, ok := r["error_count"]; ok { stats["error_count"] = v }
		if v, ok := r["avg_latency"]; ok { stats["avg_latency_ms"] = v }
	}
	return stats
}

func (a *Auth) generateToken(username string) (string, error) {
	claims := jwt.MapClaims{
		"sub": username,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.jwtSecret)
}
