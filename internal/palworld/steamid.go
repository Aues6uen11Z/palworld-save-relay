package palworld

import (
	"encoding/binary"
	"unicode/utf16"
)

// cityhash64 is Google's CityHash64, ported from palsav._cityhash (used by
// Palworld to derive a player UID from a SteamID). All arithmetic is uint64.
const (
	chK0 = 14097894508562428199
	chK1 = 13011662864482103923
	chK2 = 11160318154034397263
)

func chRot(v uint64, s uint) uint64 {
	if s == 0 {
		return v
	}
	return (v >> s) | (v << (64 - s))
}
func chShiftMix(v uint64) uint64 { return v ^ (v >> 47) }

func chHash128to64(xlo, xhi uint64) uint64 {
	const kMul = 11376068507788127593
	a := (xlo ^ xhi) * kMul
	a ^= a >> 47
	b := (xhi ^ a) * kMul
	b ^= b >> 47
	return b * kMul
}
func chHashLen16Mul(u, v, mul uint64) uint64 {
	a := (u ^ v) * mul
	a ^= a >> 47
	b := (v ^ a) * mul
	b ^= b >> 47
	return b * mul
}
func chWeakLen32Seeds(w, x, y, z, a, b uint64) (uint64, uint64) {
	a += w
	b = chRot(b+a+z, 21)
	c := a
	a += x
	a += y
	b += chRot(a, 44)
	return a + z, b + c
}
func chWeakHash32WithSeeds(s []byte, a, b uint64) (uint64, uint64) {
	return chWeakLen32Seeds(
		binary.LittleEndian.Uint64(s[0:8]),
		binary.LittleEndian.Uint64(s[8:16]),
		binary.LittleEndian.Uint64(s[16:24]),
		binary.LittleEndian.Uint64(s[24:32]),
		a, b,
	)
}
func chBswap64(x uint64) uint64 {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], x)
	return binary.LittleEndian.Uint64(b[:])
}

func chHashLen0to16(s []byte) uint64 {
	n := len(s)
	if n >= 8 {
		mul := chK2 + uint64(n)*2
		a := binary.LittleEndian.Uint64(s[:8]) + chK2
		b := binary.LittleEndian.Uint64(s[n-8:])
		c := chRot(b, 37)*mul + a
		d := (chRot(a, 25) + b) * mul
		return chHashLen16Mul(c, d, mul)
	}
	if n >= 4 {
		mul := chK2 + uint64(n)*2
		a := uint64(binary.LittleEndian.Uint32(s[:4]))
		return chHashLen16Mul(uint64(n)+(a<<3), uint64(binary.LittleEndian.Uint32(s[n-4:])), mul)
	}
	if n > 0 {
		a := uint64(s[0])
		b := uint64(s[n>>1])
		c := uint64(s[n-1])
		y := uint32(a + (b << 8))
		z := uint32(uint32(n) + uint32(c<<2))
		return chShiftMix(uint64(y)*chK2^uint64(z)*chK0) * chK2
	}
	return chK2
}

func chHashLen17to32(s []byte) uint64 {
	n := len(s)
	mul := chK2 + uint64(n)*2
	a := binary.LittleEndian.Uint64(s[:8]) * chK1
	b := binary.LittleEndian.Uint64(s[8:16])
	c := binary.LittleEndian.Uint64(s[n-8:]) * mul
	d := binary.LittleEndian.Uint64(s[n-16:n-8]) * chK2
	return chHashLen16Mul(
		chRot(a+b, 43)+chRot(c, 30)+d,
		a+chRot(b+chK2, 18)+c, mul,
	)
}

func chHashLen33to64(s []byte) uint64 {
	n := len(s)
	mul := chK2 + uint64(n)*2
	a := binary.LittleEndian.Uint64(s[:8]) * chK2
	b := binary.LittleEndian.Uint64(s[8:16])
	c := binary.LittleEndian.Uint64(s[n-24 : n-16])
	d := binary.LittleEndian.Uint64(s[n-32 : n-24])
	e := binary.LittleEndian.Uint64(s[16:24]) * chK2
	f := binary.LittleEndian.Uint64(s[24:32]) * 9
	g := binary.LittleEndian.Uint64(s[n-8:])
	h := binary.LittleEndian.Uint64(s[n-16:n-8]) * mul
	u := chRot(a+g, 43) + (chRot(b, 30)+c)*9
	v := ((a + g) ^ d) + f + 1
	w := chBswap64((u+v)*mul) + h
	x := chRot(e+f, 42) + c
	y := (chBswap64((v+w)*mul) + g) * mul
	z := e + f + c
	a = chBswap64((x+z)*mul+y) + b
	b = chShiftMix((z+a)*mul+d+h) * mul
	return b + x
}

// CityHash64 returns Google's CityHash64 of data.
func CityHash64(data []byte) uint64 {
	n := len(data)
	if n <= 32 {
		if n <= 16 {
			return chHashLen0to16(data)
		}
		return chHashLen17to32(data)
	}
	if n <= 64 {
		return chHashLen33to64(data)
	}
	x := binary.LittleEndian.Uint64(data[n-40 : n-32])
	y := binary.LittleEndian.Uint64(data[n-16:n-8]) + binary.LittleEndian.Uint64(data[n-56:n-48])
	z := chHash128to64(binary.LittleEndian.Uint64(data[n-48:n-40])+uint64(n), binary.LittleEndian.Uint64(data[n-24:n-16]))
	v0, v1 := chWeakHash32WithSeeds(data[n-64:n-32], uint64(n), z)
	w0, w1 := chWeakHash32WithSeeds(data[n-32:], y+chK1, x)
	x = x*chK1 + binary.LittleEndian.Uint64(data[:8])
	length := (n - 1) &^ 63
	pos := 0
	for length != 0 {
		x = chRot(x+y+v0+binary.LittleEndian.Uint64(data[pos+8:pos+16]), 37) * chK1
		y = chRot(y+v1+binary.LittleEndian.Uint64(data[pos+48:pos+56]), 42) * chK1
		x ^= w1
		y += v0 + binary.LittleEndian.Uint64(data[pos+40:pos+48])
		z = chRot(z+w0, 33) * chK1
		v0, v1 = chWeakHash32WithSeeds(data[pos:pos+32], v1*chK1, x+w0)
		w0, w1 = chWeakHash32WithSeeds(data[pos+32:pos+64], z+w1, y+binary.LittleEndian.Uint64(data[pos+16:pos+24]))
		z, x = x, z
		pos += 64
		length -= 64
	}
	return chHash128to64(chHash128to64(v0, w0)+chShiftMix(y)*chK1+z, chHash128to64(v1, w1)+x)
}

// SteamIDToPlayerUUID converts a SteamID64 to the Palworld player UID:
// cityhash64(steamID as UTF-16-LE) -> low 32 bits mixed -> 4 bytes + 12 zeros.
// Guests' UIDs are derived this way; the co-op host uses the 0000...0001 sentinel.
func SteamIDToPlayerUUID(steamID uint64) [16]byte {
	h := CityHash64(encodeUTF16LEString(steamID))
	lo := uint32(h)
	hi := uint32(h >> 32)
	r := lo + hi*23
	var u [16]byte
	binary.LittleEndian.PutUint32(u[:4], r)
	return u
}

// encodeUTF16LEString encodes the decimal string of v as UTF-16-LE bytes.
func encodeUTF16LEString(v uint64) []byte {
	s := uintToStr(v)
	r := []rune(s)
	u := utf16.Encode(r)
	b := make([]byte, len(u)*2)
	for i, c := range u {
		binary.LittleEndian.PutUint16(b[i*2:], c)
	}
	return b
}

func uintToStr(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
