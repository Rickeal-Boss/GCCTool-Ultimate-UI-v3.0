package client

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"math/big"
	"strings"
	"testing"
)

// fakePubKeyWithBits 构造一个指定模长的 RSA 公钥（不要求是真正的素数乘积），
// 仅用于校验强度检查逻辑本身。
func fakePubKeyWithBits(bits, e int) *rsa.PublicKey {
	n := new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
	return &rsa.PublicKey{N: n, E: e}
}

// TestValidateRSAPublicKey_ModulusBits 弱密钥必须被拒，真实部署（1024 位）必须放行
func TestValidateRSAPublicKey_ModulusBits(t *testing.T) {
	cases := []struct {
		name    string
		bits    int
		wantErr bool
	}{
		{"256位弱密钥应拒绝", 256, true},
		{"384位弱密钥应拒绝", 384, true},
		{"511位仍不足下限应拒绝", 511, true},
		{"512位达到下限应接受", 512, false},
		{"1024位正方典型部署应接受", 1024, false},
		{"2048位应接受", 2048, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRSAPublicKey(fakePubKeyWithBits(tc.bits, 65537))
			if tc.wantErr && err == nil {
				t.Fatalf("期望拒绝 %d 位密钥，实际通过了", tc.bits)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("期望接受 %d 位密钥，实际被拒: %v", tc.bits, err)
			}
		})
	}
}

// TestValidateRSAPublicKey_BadExponent 畸形指数必须被拒（偶数指数会导致不可逆）
func TestValidateRSAPublicKey_BadExponent(t *testing.T) {
	for _, tc := range []struct {
		name string
		e    int
	}{{"E=0", 0}, {"E=1", 1}, {"E=2偶数", 2}, {"E=4偶数", 4}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateRSAPublicKey(fakePubKeyWithBits(1024, tc.e)); err == nil {
				t.Fatalf("期望拒绝指数 %d，实际通过了", tc.e)
			}
		})
	}
}

func TestValidateRSAPublicKey_NilModulus(t *testing.T) {
	if err := validateRSAPublicKey(&rsa.PublicKey{N: nil, E: 65537}); err == nil {
		t.Fatal("期望拒绝空 modulus，实际通过了")
	}
}

// TestEncryptWithRSA_RejectsWeakPublicKey 端到端验证：
// encryptWithRSA 是所有公钥来源（API / 登录页内联 / PEM）的唯一收口，必须拦住弱密钥，
// 绝不能拿攻击者替换的短密钥去加密密码。
func TestEncryptWithRSA_RejectsWeakPublicKey(t *testing.T) {
	der, err := x509.MarshalPKIXPublicKey(fakePubKeyWithBits(256, 65537))
	if err != nil {
		t.Fatalf("构造弱公钥失败: %v", err)
	}
	_, err = encryptWithRSA(base64.StdEncoding.EncodeToString(der), "test-password")
	if err == nil {
		t.Fatal("期望 encryptWithRSA 拒绝 256 位弱公钥，实际加密成功了")
	}
	if !strings.Contains(err.Error(), "安全下限") {
		t.Fatalf("错误信息应说明是弱密钥，实际: %v", err)
	}
}

// TestEncryptWithRSA_AcceptsRealisticKey 防误伤：
// 正方 V9 典型部署是 1024 位密钥，加固后必须仍能正常加密。
func TestEncryptWithRSA_AcceptsRealisticKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("生成测试密钥失败: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("序列化公钥失败: %v", err)
	}
	out, err := encryptWithRSA(base64.StdEncoding.EncodeToString(der), "test-password")
	if err != nil {
		t.Fatalf("1024 位真实密钥被误拒（会直接导致登录不可用）: %v", err)
	}
	if out == "" {
		t.Fatal("加密结果为空")
	}
}
