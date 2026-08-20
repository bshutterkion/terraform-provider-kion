# terraform-kion-gcp-account

Terraform module for `kion_gcp_account`, generated from the provider schema by
`kgen module`. Do not edit by hand -- regenerate instead.

## Usage

```hcl
module "gcp_account" {
  source = "..."

  account_name            = "example"
  google_cloud_project_id = "example"
  payer_id                = 1
  project_id              = 1
  start_datecode          = "2026-01"
}
```

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
| ---- | ------- |
| terraform | >= 1.0 |
| kion | 1.0.0 |

## Providers

| Name | Version |
| ---- | ------- |
| kion | 1.0.0 |

## Modules

No modules.

## Resources

| Name | Type |
| ---- | ---- |
| [kion_gcp_account.this](https://registry.terraform.io/providers/kionsoftware/kion/1.0.0/docs/resources/gcp_account) | resource |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| account\_name | Name of the account in the application. | `string` | n/a | yes |
| google\_cloud\_project\_id | Google Cloud Project ID. Can be found in the project id field here: https://console.cloud.google.com/iam-admin/settings | `string` | n/a | yes |
| payer\_id | ID of the payer (aka Google Cloud Billing Account) | `number` | n/a | yes |
| project\_id | ID of the project where the account is attached. | `number` | n/a | yes |
| start\_datecode | Date when the GCP org will begin submitting payments against a funding source (YYYY-MM) | `string` | n/a | yes |
| account\_alias | Alias of the account in the application. | `string` | `null` | no |
| account\_type\_id | The AccountTypeID is the corresponding account's type. Will default to 15 for GCP if not given. | `number` | `null` | no |
| skip\_access\_checking | When true, the application does not perform periodic access validation. Default is false. | `bool` | `null` | no |

## Outputs

| Name | Description |
| ---- | ----------- |
| id | id of the kion\_gcp\_account. |
<!-- END_TF_DOCS -->
