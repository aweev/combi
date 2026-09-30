package catalog

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error

	// Items
	CreateItem(ctx context.Context, item *Item) error
	GetItem(ctx context.Context, id uuid.UUID) (*Item, error)
	GetItemBySlug(ctx context.Context, slug string) (*Item, error)
	ListItems(ctx context.Context, filter ItemFilter) ([]*Item, int, error)
	UpdateItem(ctx context.Context, item *Item) error
	DeleteItem(ctx context.Context, id uuid.UUID) error

	// Categorization
	AddItemToCategory(ctx context.Context, itemID, categoryID uuid.UUID) error
	RemoveAllCategoriesFromItem(ctx context.Context, itemID uuid.UUID) error
	GetCategoryIDsForItem(ctx context.Context, itemID uuid.UUID) ([]uuid.UUID, error)

	// Categories
	CreateCategory(ctx context.Context, category *Category) error
	GetCategory(ctx context.Context, id uuid.UUID) (*Category, error)
	ListCategories(ctx context.Context) ([]*Category, error)
	UpdateCategory(ctx context.Context, category *Category) error
	DeleteCategory(ctx context.Context, id uuid.UUID) error
	CountItemsInCategory(ctx context.Context, categoryID uuid.UUID) (int, error)

	// Variants
	ListVariants(ctx context.Context, itemID uuid.UUID) ([]*Variant, error)
	// (Create/Update variant methods omitted for brevity, implied similar to Item)

	// Applications
	CreateApplication(ctx context.Context, app *Application) error
	GetApplication(ctx context.Context, id uuid.UUID) (*Application, error)
	GetApplicationByUserAndItem(ctx context.Context, userID, itemID uuid.UUID) (*Application, error)
	ListApplications(ctx context.Context, itemID uuid.UUID, filter ApplicationFilter) ([]*Application, int, error)
	ListUserApplications(ctx context.Context, userID uuid.UUID) ([]*Application, error)
	UpdateApplication(ctx context.Context, app *Application) error
}
