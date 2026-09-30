package campaign

import "github.com/combi/pkg/domain/core"

const (
	PermCampaignManage   core.PermissionName = "campaign:manage"   // Admin
	PermFundraiserManage core.PermissionName = "fundraiser:manage" // Owner of the P2P page
)
