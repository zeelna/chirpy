#1. Create a new table
```
cd sql/schema
touch 0002_chirps.sql
```

#2. Create Goose migrations
```
-- +goose up
CREATE TABLE chirps(
      id UUID PRIMARY KEY,
      created_at TIMESTAMP NOT NULL,
      updated_at TIMESTAMP NOT NULL,
      body TEXT NOT NULL,

      user_id UUID NOT NULL,
      CONSTRAINT no_orphaned_chirps
          FOREIGN KEY (user_id)
              REFERENCES users(id)
              ON DELETE CASCADE
);

-- +goose down
DROP TABLE chirps;
```

#3. Run the up migration from schema directory
```
cd sql/schema
goose postgres postgres://postgres:postgres@localhost:5432/chirpy up
```

#3b. Optional - Run the down migration from schema directory
```
cd sql/schema
goose postgres postgres://postgres:postgres@localhost:5432/chirpy down
```

#3c. Verify update by connecting to DB.
```
psql "postgres://postgres:postgres@localhost:5432/chirpy"
```
#3d Check table is updated
```
SELECT * from users;
```
#4. Write the SQL queries
```
cd sql/queries
touch chirps.sql
```

```
-- name: CreateChirp :one
INSERT INTO chirps(id, created_at, updated_at, body, user_id)
VALUES (
gen_random_uuid(),
NOW(),
NOW(),
$1,
$2
)
RETURNING *;
```

#5. Use SQLC to generate Go code
```
cd <project_root>
sqlc generate
```

#6. Verify ORM-line Go code is generated
```

cd internal/database
cat chirps.sql.go
```

#7. Leverage SQLC generated code to use SQL 
```
func main(){
    //... some logic
   }
   
func (cfg *apiConfig) handlerCreateChirp(w http.ResponseWriter, req *http.Request) {
// Define struct to map JSON into variables 
    type reqParameters struct {
        Body string `json:"body"`
        UserID string `json:"user_id"`
    }

    // Decode JSON Request Body
    decoder := json.NewDecoder(req.Body)
    reqParams := reqParameters{}
    errorEncoding := decoder.Decode(&reqParams)
    // -- bad path --
    if errorEncoding != nil {
        log.Printf("Error decoding parameters: %s", errorEncoding)
        _respondWithError(w, http.StatusInternalServerError, "Something went wrong")
        return
    }
    
    ctx := req.Context() // or isntead of ctx, use context.Background()
    chirp, err := cfg.db.CreateUser(ctx, reqParams.Email)
    if errorEncoding != nil {
        _respondWithError(w, http.StatusInternalServerError, "Something went wrong")
        return
    }
}

```

