package repository

import (
	"clase02-mongo/internal/dao"
	"clase02-mongo/internal/domain"
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// MongoItemsRepository implementa ItemsRepository usando MongoDB
type MongoItemsRepository struct {
	col *mongo.Collection // Referencia a la colección "items" en MongoDB
}

// NewMongoItemsRepository crea una nueva instancia del repository
// Recibe una referencia a la base de datos MongoDB
func NewMongoItemsRepository(db *mongo.Database) MongoItemsRepository {
	return MongoItemsRepository{
		col: db.Collection("items"), // Conecta con la colección "items"
	}
}

// List obtiene todos los items de MongoDB
func (r *MongoItemsRepository) List(ctx context.Context) ([]domain.Item, error) {
	// ⏰ Timeout para evitar que la operación se cuelgue
	// Esto es importante en producción para no bloquear indefinidamente
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 🔍 Find() sin filtros retorna todos los documentos de la colección
	// bson.M{} es un filtro vacío (equivale a {} en MongoDB shell)
	cur, err := r.col.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx) // ⚠️ IMPORTANTE: Siempre cerrar el cursor para liberar recursos

	// 📦 Decodificar resultados en slice de DAO (modelo MongoDB)
	// Usamos el modelo DAO porque maneja ObjectID y tags BSON
	var daoItems []dao.Item
	if err := cur.All(ctx, &daoItems); err != nil {
		return nil, err
	}

	// 🔄 Convertir de DAO a Domain (para la capa de negocio)
	// Separamos los modelos: DAO para MongoDB, Domain para lógica de negocio
	domainItems := make([]domain.Item, len(daoItems))
	for i, daoItem := range daoItems {
		domainItems[i] = daoItem.ToDomain() // Función definida en dao/Item.go
	}

	return domainItems, nil
}

// Create inserta un nuevo item en MongoDB
// Consigna 1: Validar name y price >= 0, agregar timestamps
func (r *MongoItemsRepository) Create(ctx context.Context, item domain.Item) (domain.Item, error) {
	if item.Name == "" {
		return domain.Item{}, errors.New("name no puede estar vacío")
	}
	if item.Price < 0 {
		return domain.Item{}, errors.New("price no puede ser negativo")
	}

	mongoItem := dao.Item{
		Name:      item.Name,
		Price:     item.Price,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_, err := r.col.InsertOne(ctx, mongoItem)
	if err != nil {
		return domain.Item{}, err
	}
	return item, nil
}

// GetByID busca un item por su ID
// Consigna 2: Validar que el ID sea un ObjectID válido
func (r *MongoItemsRepository) GetByID(ctx context.Context, id string) (domain.Item, error) {
	idHEX, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return domain.Item{}, errors.New("invalid id format")
	}
	var daoItem dao.Item
	err = r.col.FindOne(ctx, bson.M{"_id": idHEX}).Decode(&daoItem)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return domain.Item{}, errors.New("item not found")
		}
		return domain.Item{}, err
	}
	return daoItem.ToDomain(), nil
}

// Update actualiza un item existente
// Consigna 3: Update parcial + actualizar updatedAt
func (r *MongoItemsRepository) Update(ctx context.Context, id string, item domain.Item) (domain.Item, error) {
	idHEX, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return domain.Item{}, errors.New("invalid id format")
	}

	updateFields := bson.M{}
	if item.Name != "" {
		updateFields["name"] = item.Name
	}
	if item.Price != 0 {
		updateFields["price"] = item.Price
	}
	updateFields["updatedAt"] = time.Now()

	update := bson.M{"$set": updateFields}

	res, err := r.col.UpdateByID(ctx, idHEX, update)
	if err != nil {
		return domain.Item{}, err
	}
	if res.MatchedCount == 0 {
		return domain.Item{}, errors.New("item not found")
	}

	// Opcional: devolver el item actualizado consultando de nuevo
	var daoItem dao.Item
	err = r.col.FindOne(ctx, bson.M{"_id": idHEX}).Decode(&daoItem)
	if err != nil {
		return domain.Item{}, err
	}
	return daoItem.ToDomain(), nil
}

// Delete elimina un item por ID
// Consigna 4: Eliminar documento de MongoDB
func (r *MongoItemsRepository) Delete(ctx context.Context, id string) error {
	idHEX, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid id format")
	}
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": idHEX})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return errors.New("item not found")
	}
	return nil
}
