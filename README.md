# terra-ci
CI for Terraform

# Features
```
./terra-ci module create --path modules//terra-ci
./terra-ci module test --path modules//terra-ci

./terra-ci workspace create --path live/_global/account-baseline

./terra-ci workspace plan --path live/_global/account-baseline -out tfplan -destroy
./terra-ci workspace plan --local --path live/_global/account-baseline -out tfplan -destroy
./terra-ci workspace plan --local --source modules//terraform-state --path live/_global/account-baseline -out tfplan -destroy
./terra-ci workspace plan --path live/_global/account-baseline -out tfplan
./terra-ci workspace plan --local --path live/_global/account-baseline -out tfplan
./terra-ci workspace plan --local --source modules//terraform-state --path live/_global/account-baseline -out tfplan
./terra-ci workspace plan --path live/_global/account-baseline
./terra-ci workspace plan --local --path live/_global/account-baseline
./terra-ci workspace plan --local --source module//terraform-state --path live/_global/account-baseline

./terra-ci workspace apply --path live/_global/account-baseline
./terra-ci workspace apply --local --path live/_global/account-baseline
./terra-ci workspace apply --local --source modules//account-baseline --path live/_global/account-baseline
./terra-ci workspace apply --path live/_global/account-baseline tfplan
./terra-ci workspace apply --local --path live/_global/account-baseline tfplan
./terra-ci workspace apply --local --source modules//account-baseline --path live/_global/account-baseline tfplan

./terra-ci workspace destroy --path live/_global/account-baseline
./terra-ci workspace destroy --local --path live/_global/account-baseline
./terra-ci workspace destroy --local --source modules//account-baseline --path live/_global/account-baseline

./terra-ci workspace delete --path live/_global/account-baseline

./terra-ci workspace revert --path live/_global/account-baseline --ref 834c3114333294d4aad6ab348fe9c8fb105f25af
```
