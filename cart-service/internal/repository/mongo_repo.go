package repository

import (
	"context"
	"log/slog"
	"time"

	"github.com/toainguyen/ecommerce/cart-service/internal/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const cartCollection = "carts"

// MongoRepository implements CartRepository with the official mongo-go-driver.
type MongoRepository struct {
	coll *mongo.Collection
	log  *slog.Logger
}

// NewMongoRepository connects to MongoDB, pings it (connection stub), and returns the repo.
func NewMongoRepository(uri, dbName string, log *slog.Logger) (*MongoRepository, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Error("mongo connect failed", "error", err)
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		log.Error("mongo ping failed", "error", err)
		return nil, err
	}
	log.Info("connected to mongodb", "db", dbName)

	return &MongoRepository{coll: client.Database(dbName).Collection(cartCollection), log: log}, nil
}

// Upsert replaces (or inserts) the whole cart document for a user.
func (r *MongoRepository) Upsert(ctx context.Context, cart *model.Cart) error {
	cart.UpdatedAt = time.Now()
	_, err := r.coll.ReplaceOne(
		ctx,
		bson.M{"_id": cart.UserID},
		cart,
		options.Replace().SetUpsert(true),
	)
	return err
}

func (r *MongoRepository) Get(ctx context.Context, userID string) (*model.Cart, error) {
	var cart model.Cart
	err := r.coll.FindOne(ctx, bson.M{"_id": userID}).Decode(&cart)
	if err == mongo.ErrNoDocuments {
		// Empty cart is a valid state.
		return &model.Cart{UserID: userID, Items: []model.CartItem{}}, nil
	}
	if err != nil {
		return nil, err
	}
	return &cart, nil
}
