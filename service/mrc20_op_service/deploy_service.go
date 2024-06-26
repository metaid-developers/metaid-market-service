package mrc20_op_service

import (
	"metaid-market-service/controller/request"
	"metaid-market-service/controller/respond"
	"metaid-market-service/models"
	"metaid-market-service/service/parse_service"
)

func Mrc20Deploy(req *request.Mrc20DeployRequest) (*respond.Mrc20DeployResp, error) {
	var (
		entity *models.Mrc20DeployOrderModel
		err    error

		pin *parse_service.PersonalInformationNode
	)

}
