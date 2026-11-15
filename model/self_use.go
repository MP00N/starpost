package model

// 站长的自用流量不是生意。
//
// 超级管理员（role >= RoleRootUser）调用自己的站点，钱从左手出到右手：既不是
// 收入，也不该算利润。把它混进盈亏分析里，会让整页数字失真——实测一个只有
// 五个用户的站点，站长自己占了 99.99% 的消费额，所有「毛利率」「亏损模型」
// 都在描述站长自己的试用记录，真实客户的 0.003 美元被彻底淹没。
//
// 对账同理：对账要回答的是「客户被扣的钱和我算的对不对」，站长自己那一笔
// 扣对扣错都不影响生意。

import "github.com/QuantumNous/new-api/common"

// SelfUseUserIds returns the ids of users whose traffic is the operator's own.
//
// The role is read from the database rather than the request context on purpose:
// the caller is asking about *history*, and a user promoted or demoted after a
// request was served should be counted by what they were not by what they are
// now. A log row also cannot tell us the role at the time, so the current role
// is the best available answer — and for the root account it never changes in
// practice.
func SelfUseUserIds() []int {
	var ids []int
	// Root only. Administrators (role 10) are staff, but their usage may still
	// be real demand (an admin testing a customer's complaint, for example);
	// only the site owner is definitionally spending his own money.
	err := DB.Model(&User{}).
		Where("role >= ?", common.RoleRootUser).
		Pluck("id", &ids).Error
	if err != nil {
		return nil
	}
	return ids
}
