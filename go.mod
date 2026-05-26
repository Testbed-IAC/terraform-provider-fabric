module github.com/Testbed-IAC/terraform-provider-fabric

go 1.24

require (
	github.com/Testbed-IAC/fabric-go-fim v0.0.0
	github.com/Testbed-IAC/fabric-orchestrator-go-client v0.0.0
	github.com/hashicorp/terraform-plugin-framework v1.13.0
	github.com/hashicorp/terraform-plugin-framework-timeouts v0.5.0
	github.com/hashicorp/terraform-plugin-framework-validators v0.15.0
	github.com/hashicorp/terraform-plugin-go v0.25.0
	github.com/hashicorp/terraform-plugin-log v0.9.0
	github.com/hashicorp/terraform-plugin-testing v1.11.0
)

replace (
	github.com/Testbed-IAC/fabric-go-fim => ../fabric-go-fim
	github.com/Testbed-IAC/fabric-orchestrator-go-client => ../fabric-orchestrator-go-client
)
