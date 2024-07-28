package word

import (
	"testing"
	"time"
	"math/rand"
)

func TestPalindrome(t *testing.T) {
	if !IsPalindrome("detartrated") {
		t.Error(`IsPalindrome("detartrated")=false`)
	}
	if !IsPalindrome("kayak") {
		t.Error(`IsPalindrome("kayak")=false`)
	}
}
func TestNonPalindrome(t *testing.T) {
	if IsPalindrome("palindrome") {
		t.Error(`IsPalindrome("palindrome") = true`)
	}
}
func TestFreachPalindrome(t *testing.T) {
	if !IsPalindrome("été") {
		t.Error(`IsPalindrome("été") = false`)
	}
}
func TestCanalPalindrome(t *testing.T) {
	input := "A man, a plan, a canal: Panama"
	if !IsPalindrome(input) {
		t.Errorf(`IsPalindrome(%q) = false`, input)
	}
}

func randomPalindrome(rng *rand.Rand) string {
	n := rng.Intn(25)
	runes := make([]rune, n)
	for i := 0; i < (n+1)/2; i++ {
		r := rune(rng.Intn(0x1000))
		runes[i] = r
		runes[n-i-1] = r
	}
	return string(runes)
}
func TestRandomPalindrimes(t *testing.T){
	seed:=time.Now().UTC().UnixNano()
	t.Logf("random seed: %d",seed)
	rng:=rand.New(rand.NewSource(seed))

	for i:=0;i<1000;i++{
		p:=randomPalindrome(rng)
		if !IsPalindrome(p){
			t.Errorf("IsPalindrome(%q) = false",p)
		}
	}
}
