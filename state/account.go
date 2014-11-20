package state

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/tendermint/go-ed25519"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
)

const (
	AccountStatusNominal   = byte(0x00)
	AccountStatusBonded    = byte(0x01)
	AccountStatusUnbonding = byte(0x02)
	AccountStatusDupedOut  = byte(0x03)
)

type Account struct {
	Id     uint64 // Numeric id of account, incrementing.
	PubKey []byte
}

func ReadAccount(r io.Reader, n *int64, err *error) Account {
	return Account{
		Id:     ReadUInt64(r, n, err),
		PubKey: ReadByteSlice(r, n, err),
	}
}

func (account Account) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt64(w, account.Id, &n, &err)
	WriteByteSlice(w, account.PubKey, &n, &err)
	return
}

func (account Account) VerifyBytes(msg []byte, sig Signature) bool {
	if sig.SignerId != account.Id {
		panic("account.id doesn't match sig.signerid")
	}
	if len(sig.Bytes) == 0 {
		panic("signature is empty")
	}
	v1 := &ed25519.Verify{
		Message:   msg,
		PubKey:    account.PubKey,
		Signature: sig.Bytes,
	}
	ok := ed25519.VerifyBatch([]*ed25519.Verify{v1})
	return ok
}

func (account Account) Verify(o Signable) bool {
	sig := o.GetSignature()
	o.SetSignature(Signature{}) // clear
	msg := BinaryBytes(o)
	o.SetSignature(sig) // restore
	return account.VerifyBytes(msg, sig)
}

func (account Account) String() string {
	return fmt.Sprintf("Account{%v:%X}", account.Id, account.PubKey[:6])
}

//-----------------------------------------------------------------------------

type AccountDetail struct {
	Account
	Sequence uint
	Balance  uint64
	Status   byte
}

func ReadAccountDetail(r io.Reader, n *int64, err *error) *AccountDetail {
	return &AccountDetail{
		Account:  ReadAccount(r, n, err),
		Sequence: ReadUVarInt(r, n, err),
		Balance:  ReadUInt64(r, n, err),
		Status:   ReadByte(r, n, err),
	}
}

func (accDet *AccountDetail) WriteTo(w io.Writer) (n int64, err error) {
	WriteBinary(w, accDet.Account, &n, &err)
	WriteUVarInt(w, accDet.Sequence, &n, &err)
	WriteUInt64(w, accDet.Balance, &n, &err)
	WriteByte(w, accDet.Status, &n, &err)
	return
}

func (accDet *AccountDetail) String() string {
	return fmt.Sprintf("AccountDetail{%v:%X Sequence:%v Balance:%v Status:%X}",
		accDet.Id, accDet.PubKey, accDet.Sequence, accDet.Balance, accDet.Status)
}

func (accDet *AccountDetail) Copy() *AccountDetail {
	accDetCopy := *accDet
	return &accDetCopy
}

//-------------------------------------

var AccountDetailCodec = accountDetailCodec{}

type accountDetailCodec struct{}

func (abc accountDetailCodec) Encode(accDet interface{}, w io.Writer, n *int64, err *error) {
	WriteBinary(w, accDet.(*AccountDetail), n, err)
}

func (abc accountDetailCodec) Decode(r io.Reader, n *int64, err *error) interface{} {
	return ReadAccountDetail(r, n, err)
}

func (abc accountDetailCodec) Compare(o1 interface{}, o2 interface{}) int {
	panic("AccountDetailCodec.Compare not implemented")
}

//-----------------------------------------------------------------------------

type PrivAccount struct {
	Account
	PrivKey []byte
}

// Generates a new account with private key.
// The Account.Id is empty since it isn't in the blockchain.
func GenPrivAccount() *PrivAccount {
	privKey := CRandBytes(32)
	pubKey := ed25519.MakePubKey(privKey)
	return &PrivAccount{
		Account: Account{
			Id:     uint64(0),
			PubKey: pubKey,
		},
		PrivKey: privKey,
	}
}

// The Account.Id is empty since it isn't in the blockchain.
func PrivAccountFromJSON(jsonBlob []byte) (privAccount *PrivAccount) {
	err := json.Unmarshal(jsonBlob, &privAccount)
	if err != nil {
		Panicf("Couldn't read PrivAccount: %v", err)
	}
	return
}

// The Account.Id is empty since it isn't in the blockchain.
func PrivAccountFromFile(file string) *PrivAccount {
	jsonBlob, err := ioutil.ReadFile(file)
	if err != nil {
		Panicf("Couldn't read PrivAccount from file: %v", err)
	}
	return PrivAccountFromJSON(jsonBlob)
}

func (pa *PrivAccount) SignBytes(msg []byte) Signature {
	signature := ed25519.SignMessage(msg, pa.PrivKey, pa.PubKey)
	sig := Signature{
		SignerId: pa.Id,
		Bytes:    signature,
	}
	return sig
}

func (pa *PrivAccount) Sign(o Signable) {
	if !o.GetSignature().IsZero() {
		panic("Cannot sign: already signed")
	}
	msg := BinaryBytes(o)
	sig := pa.SignBytes(msg)
	o.SetSignature(sig)
}