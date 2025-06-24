package services

import (
	test "MPHEDev/pkg/testFunc"

	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

func (c *Coordinator) TestPublicKey() {
	if c.globalPK != nil && c.skAgg != nil {
		encoder := ckks.NewEncoder(c.params)
		encryptor := ckks.NewEncryptor(c.params, c.globalPK)
		decryptorAgg := ckks.NewDecryptor(c.params, c.skAgg)
		test.TestPublicKey(c.params, encoder, encryptor, decryptorAgg)
	}
}
