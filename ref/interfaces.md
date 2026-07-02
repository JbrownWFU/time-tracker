```go
package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // Pure-Go SQLite driver
)

// ==========================================
// 1. THE DATA MODEL & THE INTERFACE (The Contract)
// ==========================================

type User struct {
	ID   int
	Name string
}

// UserRepository defines the rules. Anyone who wants to be a UserRepository
// MUST have these two exact methods.
type UserRepository interface {
	Save(u User) error
	GetByID(id int) (User, error)
}

// ==========================================
// 2. IMPLEMENTATION #1: THE REAL SQLITE DB
// ==========================================

type SQLiteRepo struct {
	db *sql.DB
}

func NewSQLiteRepo(dbPath string) (*SQLiteRepo, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	// Create table if it doesn't exist
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT);`)
	if err != nil {
		return nil, err
	}
	return &SQLiteRepo{db: db}, nil
}

func (r *SQLiteRepo) Save(u User) error {
	_, err := r.db.Exec("INSERT OR REPLACE INTO users (id, name) VALUES (?, ?)", u.ID, u.Name)
	return err
}

func (r *SQLiteRepo) GetByID(id int) (User, error) {
	var u User
	err := r.db.QueryRow("SELECT id, name FROM users WHERE id = ?", id).Scan(&u.ID, &u.Name)
	return u, err
}

// ==========================================
// 3. IMPLEMENTATION #2: THE FAKE IN-MEMORY DB (For Testing)
// ==========================================

type FakeInMemoryRepo struct {
	store map[int]User
}

func NewFakeInMemoryRepo() *FakeInMemoryRepo {
	return &FakeInMemoryRepo{
		store: make(map[int]User),
	}
}

func (f *FakeInMemoryRepo) Save(u User) error {
	f.store[u.ID] = u
	return nil
}

func (f *FakeInMemoryRepo) GetByID(id int) (User, error) {
	user, exists := f.store[id]
	if !exists {
		return User{}, fmt.Errorf("user not found")
	}
	return user, nil
}

// ==========================================
// 4. THE CONSUMER (The Business Logic)
// ==========================================

// UserService doesn't know about SQLite OR the Fake memory map.
// It only knows about the UserRepository interface.
type UserService struct {
	repo UserRepository
}

func (s *UserService) RunBusinessLogic() {
	// 1. Save a user
	u := User{ID: 42, Name: "Devin"}
	_ = s.repo.Save(u)

	// 2. Fetch the user back
	fetched, _ := s.repo.GetByID(42)
	fmt.Printf("Successfully fetched user '%s' from the repository!\n", fetched.Name)
}

// ==========================================
// 5. THE EXECUTION
// ==========================================

func main() {
	// --- RUNNING WITH THE REAL SQLITE DATABASE ---
	fmt.Println("--- Scenario A: Production (Using Real SQLite) ---")
	
	realDB, err := NewSQLiteRepo("production.db")
	if err != nil {
		panic(err)
	}
	
	// Inject the real SQLite repo into the service
	prodService := UserService{repo: realDB}
	prodService.RunBusinessLogic()

	// --- RUNNING WITH THE FAKE IN-MEMORY MAP ---
	fmt.Println("\n--- Scenario B: Testing (Using Fake In-Memory Map) ---")
	
	fakeDB := NewFakeInMemoryRepo()
	
	// Inject the fake map repo into the exact same service struct
	testService := UserService{repo: fakeDB}
	testService.RunBusinessLogic()
}
```