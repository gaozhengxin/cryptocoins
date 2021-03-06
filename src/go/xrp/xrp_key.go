package xrp
import(
	"encoding/hex"
	"math/big"

	"github.com/rubblelabs/ripple/crypto"
)

const (
	PubKeyBytesLenCompressed   = 33
	PubKeyBytesLenUncompressed = 65
)

const (
	pubkeyCompressed   byte = 0x2
	pubkeyUncompressed byte = 0x4
)

// cryptoType = "ed25519" or "ecdsa"
func XRP_importKeyFromSeed(seed string, cryptoType string) crypto.Key {
	shash, err := crypto.NewRippleHashCheck(seed, crypto.RIPPLE_FAMILY_SEED)
	checkErr(err)
	switch cryptoType {
	case "ed25519":
		key, _ := crypto.NewEd25519Key(shash.Payload())
		return key
	case "ecdsa":
		key, _ := crypto.NewECDSAKey(shash.Payload())
		return key
	default:
		return nil
	}
}

func XRP_publicKeyToAddress(pubkey []byte) string {
	return XRP_getAddress(XRP_importPublicKey(pubkey), nil)
}

func XRP_importPublicKey(pubkey []byte) crypto.Key {
	return &EcdsaPublic{pub:pubkey,}
}

type EcdsaPublic struct {
	pub []byte
}

func XRP_getAddress(k crypto.Key, sequence *uint32) string {
	prefix := []byte{0}
	address := crypto.Base58Encode(append(prefix, k.Id(sequence)...), crypto.ALPHABET)
        return address
}

func (k *EcdsaPublic) Id(sequence *uint32) []byte {
	return crypto.Sha256RipeMD160(k.Public(sequence))
}

func (k *EcdsaPublic) Private(sequence *uint32) []byte {
	return nil
}

func (k *EcdsaPublic) Public(sequence *uint32) []byte {
	if len(k.pub) == PubKeyBytesLenCompressed {
		return k.pub
	} else {
		xs := hex.EncodeToString(k.pub[1:33])
		ys := hex.EncodeToString(k.pub[33:])
		x, _ := new(big.Int).SetString(xs,16)
		y, _ := new(big.Int).SetString(ys,16)
		b := make([]byte, 0, PubKeyBytesLenCompressed)
		format := pubkeyCompressed
		if isOdd(y) {
			format |= 0x1
		}
		b = append(b, format)
		return paddedAppend(32, b, x.Bytes())
	}
}

func isOdd(a *big.Int) bool {
	return a.Bit(0) == 1
}

func paddedAppend(size uint, dst, src []byte) []byte {
	for i := 0; i < int(size)-len(src); i++ {
		dst = append(dst, 0)
	}
	return append(dst, src...)
}
