package domain

import "crypto/rand"

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func NewCode(size int) (string, error) {
	if size < 1 || size > 32 {
		return "", ErrInvalidCodeSize
	}
	result := make([]byte, size)
	buf := make([]byte, size*2)
	for filled := 0; filled < size; {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, value := range buf {
			if value >= 248 {
				continue
			}
			result[filled] = alphabet[int(value)%len(alphabet)]
			filled++
			if filled == size {
				break
			}
		}
	}
	return string(result), nil
}
