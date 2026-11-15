package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 站长的自用流量不能进盈亏/对账。这个站点上它占了 99.99% 的消费额，混进去
// 之后每一列数字都在描述站长自己的试用记录。
func TestSelfUseUserIdsExcludesOnlyRoot(t *testing.T) {
	previousDB := model.DB
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.User{}))
	model.DB = database
	t.Cleanup(func() { model.DB = previousDB })

	// aff_code carries a unique index and the column has no default, so each row
	// needs its own value — two empty strings collide.
	require.NoError(t, database.Create(&model.User{
		Id: 1, Username: "admin", Role: common.RoleRootUser, AffCode: "a1",
	}).Error)
	require.NoError(t, database.Create(&model.User{
		Id: 2, Username: "staff", Role: common.RoleAdminUser, AffCode: "a2",
	}).Error)
	require.NoError(t, database.Create(&model.User{
		Id: 3, Username: "customer", Role: common.RoleCommonUser, AffCode: "a3",
	}).Error)

	ids := model.SelfUseUserIds()
	require.Equal(t, []int{1}, ids,
		"only the root account is spending its own money; "+
			"admins and customers are real traffic")
}
