package aws

import (
	"fmt"
	"testing"

	"github.com/mattermost/mattermost-cloud/internal/tools/aws/mocks"
)

func Test(t *testing.T) {
	a := &Client{
		RDS: &mocks.RDSAPI{},
	}

	fmt.Print(a)

}
