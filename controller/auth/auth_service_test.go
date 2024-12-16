package auth

import (
	"fmt"
	"testing"
)

func TestSignMessage(t *testing.T) {
	message := verifyMessage
	privateKey := "b52098c3ca9768b00025fc5d511b1bba822bc3669380a1842a5a062b2ab1ec2c"
	messageSign, err := SignMessage(message, privateKey)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(messageSign)
}

func TestVerifySign(t *testing.T) {
	//message := "Hello"
	//messageSign := "304402202b9289fd72ad416bc6259b9f1a4867ec7f35377631efdc64b070ecdaaf6f481a022020e7d190eed4af2788f5f1b44ed45fbbcc9e3b933de5e22d60607c73c9648103"
	//publicKey := "0338aa9a486efebf5764499c3b5ec7bdd2e4aa0e3e824c35d68b280f00f91a0582"
	//verified, err := VerifySign(message, messageSign, publicKey)
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//fmt.Println(verified)

	message := verifyMessage
	messageSign := "G07+r4D+eX+87+5g97j3vTXWsBgEbTqjA1nGS8iuouVfPSnn65OJLEB2LZEcSiiDgkF33GaD1+3wNd3nXK5apJo="
	publicKey := "0214ce8cc53f654dff7757a7886ad9317dd2b2ea4223a130338cceb6973857e73b"
	verified, err := VerifyTextSign(message, messageSign, publicKey)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(verified)
}

func TestVerifySign2(t *testing.T) {
	//message := "Hello"
	//messageSign := "304402202b9289fd72ad416bc6259b9f1a4867ec7f35377631efdc64b070ecdaaf6f481a022020e7d190eed4af2788f5f1b44ed45fbbcc9e3b933de5e22d60607c73c9648103"
	//publicKey := "0338aa9a486efebf5764499c3b5ec7bdd2e4aa0e3e824c35d68b280f00f91a0582"
	//verified, err := VerifySign(message, messageSign, publicKey)
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//fmt.Println(verified)

	message := "TRYEE"
	messageSign := "H814FMHYKwfkK5Azc3EHIA0CYPAkGLJXRmx7D516aMPMB+ijs89iaoN26IjsPArrlKp7k30gfORcm7IlHO6bMMU="
	publicKey := "022f5ef2e3c610e1c4b3eb2ec55c5f12e3bc375cb6a16888eb720bdeb31ea7107e"
	verified, err := VerifyTextSign(message, messageSign, publicKey)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(verified)
}
