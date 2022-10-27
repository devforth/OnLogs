package requests_test

import (
	"fmt"
	"testing"

	requests "github.com/devforth/OnLogs/app/requests"
)

func TestGetContainerLogs(t *testing.T) {
	res := requests.GetContainerLogs("a", 0, 0)
	fmt.Println(res)

}
