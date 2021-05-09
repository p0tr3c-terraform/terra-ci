package templates

const (
	TerragruntWorkspaceConfig = `# Automatically generated by terra-ci
inputs = {}

terraform {
  source = "{{ .ModuleLocation }}"
}

include {
  path = find_in_parent_folders()
}`
	CiWorkspaceConfigTpl = `# Automatically generate by terra-ci
name: Run - Terraform Plan for {{ .WorkspaceName }}
on:
  workflow_dispatch:
  push:
    branches-ignore:
      - {{ .WorkspaceDefaultProdBranch }}
    paths:
      - {{ .WorkspacePath }}
jobs:
  RunTerragruntPlan:
    runs-on: ubuntu-latest
    env:
      TERRA_CI_STATE_MACHINE_ARN: "{{ .WorkspaceTerragruntRunnerARN }}"
    steps:
      - uses: actions/checkout@v2
      - run: gh release download --pattern terra-ci-linux-amd
        env:
          GH_TOKEN: ${{ ` + "`{{`" + ` }} secrets.PAT {{ ` + "`}}`" + ` }}
          GH_REPO: github.com/p0tr3c-terraform/terra-ci
      - run: chmod +x terra-ci-linux-amd
      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@v1
        with:
          aws-access-key-id: ${{ ` + "`{{`" + ` }} secrets.AWS_ACCESS_KEY_ID {{ ` + "`}}`" + ` }}
          aws-secret-access-key: ${{ ` + "`{{`" + ` }} secrets.AWS_SECRET_ACCESS_KEY {{` + "`}}`" + ` }}
          aws-region: eu-west-1
      - uses: actions/setup-go@v2
        with:
          go-version: '1.15.2'
      - run: terra-ci run workspace  --path={{ .WorkspacePath }} --branch=${GITHUB_REF##*/}
`
	StateMachineInputTpl = `
{
    "Comment": "Run from CLI",
    "build": {
	  "sourceversion": "{{ .Branch }}",
	  "action": "{{ .Action }}",
      "environment": {
        "terra_ci_resource": "{{ .Resource }}"
      }
    }
}
`
)