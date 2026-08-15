package password

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !Verify(hash, "correct horse battery staple") {
		t.Fatal("Verify must accept the correct password")
	}
	if Verify(hash, "wrong password") {
		t.Fatal("Verify must reject a wrong password")
	}
	if Verify("not-a-bcrypt-hash", "x") {
		t.Fatal("Verify must reject a malformed hash")
	}
}

func TestHashUsesCost12(t *testing.T) {
	hash, err := Hash("pw")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		t.Fatalf("Cost: %v", err)
	}
	if cost != Cost {
		t.Fatalf("bcrypt cost = %d, want %d", cost, Cost)
	}
}
