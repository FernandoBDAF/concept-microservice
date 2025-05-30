# Shared Models Usage Guide

This guide demonstrates how to use the shared model system, including base models, validation, serialization, versioning, conversion, documentation generation, testing utilities, and migration utilities.

---

## 1. Base Model Usage

```go
import "github.com/FBDAF/microservices/services/common/models/domain"

type MyModel struct {
    domain.BaseModel
    Name string `json:"name"`
}

func NewMyModel(name string) *MyModel {
    return &MyModel{
        BaseModel: *domain.NewBaseModel(),
        Name: name,
    }
}
```

---

## 2. Validation

```go
import "github.com/FBDAF/microservices/services/common/models/domain"

func (m *MyModel) Validate() error {
    return domain.ValidateString(m.Name, domain.ValidationRules{
        Required: true,
        MinLength: 3,
    })
}
```

---

## 3. Serialization

```go
import (
    "encoding/json"
    "github.com/FBDAF/microservices/services/common/models/domain"
)

model := NewMyModel("example")
data, err := json.Marshal(model)
// data is JSON representation
```

---

## 4. Versioning

```go
import "github.com/FBDAF/microservices/services/common/models/domain"

type Versioned struct {
    domain.VersionedBaseModel
    Field string
}

func NewVersioned() *Versioned {
    return &Versioned{
        VersionedBaseModel: domain.VersionedBaseModel{
            BaseModel: *domain.NewBaseModel(),
            Version: domain.Version{Major: 1, Minor: 0, Patch: 0},
        },
        Field: "value",
    }
}
```

---

## 5. Conversion Utilities

```go
import "github.com/FBDAF/microservices/services/common/models/domain"

src := &MyModel{Name: "foo"}
converter := domain.BaseConverter{Model: src}
var target MyModel
err := converter.ConvertTo(&target)
```

---

## 6. Documentation Generation

```go
import (
    "fmt"
    "reflect"
    "github.com/FBDAF/microservices/services/common/models/domain"
)

generator := domain.NewDocumentationGenerator()
doc := &domain.ModelDocumentation{
    Name: "MyModel",
    Description: "Example model",
    Fields: generator.GenerateFieldDocumentation(reflect.TypeOf(MyModel{})),
}
generator.RegisterModel(reflect.TypeOf(MyModel{}), doc)
markdown := generator.GenerateMarkdown(reflect.TypeOf(MyModel{}))
fmt.Println(markdown)
```

---

## 7. Testing Utilities

```go
import (
    "testing"
    "github.com/FBDAF/microservices/services/common/models/domain"
)

func TestMyModelValidation(t *testing.T) {
    tester := domain.NewModelTester(t)
    valid := MyModel{Name: "valid"}
    invalid := MyModel{Name: ""}
    tester.TestModelValidation(&valid, []interface{}{valid}, []interface{}{invalid})
}
```

---

## 8. Migration Utilities

```go
import (
    "reflect"
    "github.com/FBDAF/microservices/services/common/models/domain"
)

type OldModel struct {
    domain.VersionedBaseModel
    Name string
}
type NewModel struct {
    domain.VersionedBaseModel
    Name string
    Email string
}

func migrateOldToNew(old interface{}) (interface{}, error) {
    o, ok := old.(*OldModel)
    if !ok {
        return nil, errors.New("invalid type")
    }
    return &NewModel{
        VersionedBaseModel: o.VersionedBaseModel,
        Name: o.Name,
        Email: "",
    }, nil
}

registry := domain.NewMigrationRegistry()
registry.RegisterMigration(reflect.TypeOf(&OldModel{}), domain.Version{Major: 1, Minor: 0, Patch: 0}, migrateOldToNew)

migrated, err := registry.Migrate(&OldModel{/* ... */}, domain.Version{Major: 2, Minor: 0, Patch: 0})
if err != nil {
    // handle error
}
newModel := migrated.(*NewModel)
```

---

## 9. Example: User Model

See `user.go` in this directory for a comprehensive example of a user model using all shared model features.
