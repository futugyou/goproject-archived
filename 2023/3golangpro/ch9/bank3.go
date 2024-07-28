package bank
import "sync"

var (
	mu      sync.Mutex
	rmu     sync.RWMutex
	balance int
)

func Deposit(amount int) {
	mu.Lock()
	balance = balance + amount
	mu.Unlock()
}
func Balance() int {
	rmu.RLock()
	defer rmu.RUnlock()
	return balance
}

func Withdraw(amount int) bool {
	mu.Lock()
	defer mu.Unlock()
	if balance >= amount {
		balance -= amount
		return true
	}
	return false
}