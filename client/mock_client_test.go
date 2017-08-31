package client

import "testing"

func TestMockClient(t *testing.T) {
	c := NewMock()
	c.AccountRegister(nil)
	c.AccountStatus()
	c.Events("")
	c.Queries(nil)
	c.KeyRequest()
	c.KeyReset(nil)
}
