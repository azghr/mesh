# bulkops

Batch insert/update for database efficiency.

## What It Does

Provides batch operations for database efficiency:
- Configurable batch size
- Automatic batching for large datasets
- Context cancellation support
- Insert, Update, and Delete operations

## Usage

### Bulk Insert

```go
users := []User{
    {Name: "john"},
    {Name: "jane"},
    {Name: "bob"},
}

err := bulkops.Insert(ctx, users, 100, func(batch []User) error {
    return db.InsertUsers(ctx, batch)
})
```

### Bulk Update

```go
users := []User{
    {ID: 1, Name: "john updated"},
    {ID: 2, Name: "jane updated"},
}

err := bulkops.Update(ctx, users, 50, func(batch []User) error {
    return db.UpdateUsers(ctx, batch)
})
```

### Bulk Delete

```go
ids := []string{"1", "2", "3", "4", "5"}

err := bulkops.Delete(ctx, ids, 100, func(batch []string) error {
    return db.DeleteUsers(ctx, batch)
})
```

### InBatches Helper

When you need more control over the batching process:

```go
users := []User{...} // large list
batches := bulkops.InBatches(users, 100)

for _, batch := range batches {
    if err := db.InsertUsers(ctx, batch); err != nil {
        return err
    }
}
```

## Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| batchSize | 100 | Number of items per batch |

Zero or negative batchSize uses default (100).

## Context Support

All operations support context cancellation:

```go
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

err := bulkops.Insert(ctx, users, 100, insertFn)
// Returns ctx.Err() on timeout
```

## Error Handling

On error, the operation stops and returns the error:

```go
err := bulkops.Insert(ctx, users, 100, func(batch []User) error {
    if err := db.InsertUsers(ctx, batch); err != nil {
        return err  // stops further processing
    }
    return nil
})
```

## Result Tracking

```go
result := bulkops.NewResult(len(users))

for i, batch := range bulkops.InBatches(users, 100) {
    if err := db.InsertUsers(ctx, batch); err != nil {
        result.AddFailed(len(batch))
        continue
    }
    result.AddProcessed(len(batch))
}

log.Printf("Processed: %d, Failed: %d", result.Processed, result.Failed)
```

## Example: Large Dataset

```go
func ImportUsers(ctx context.Context, users []User) error {
    return bulkops.Insert(ctx, users, 100, func(batch []User) error {
        return db.WithTx(ctx, func(tx *sql.Tx) error {
            return bulkops.InsertTx(ctx, tx, batch)
        })
    })
}
```

## Performance Tips

1. **Batch size**: 100-500 typically optimal
2. **Transaction**: Wrap batches in a single transaction
3. **Context**: Use context with timeout for large imports
4. **Logging**: Track progress for large datasets

## With Database Pool

```go
func bulkInsert(db *sql.DB, users []User) error {
    return bulkops.Insert(context.Background(), users, 100, func(batch []User) error {
        _, err := db.ExecContext(context.Background(), `
            INSERT INTO users (name) VALUES 
            `+strings.Join(makePlaceholders(len(batch)), ",")+`
        `, args...)
        return err
    })
}
```