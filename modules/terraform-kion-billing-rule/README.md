# terraform-kion-billing-rule

Terraform module for `kion_billing_rule`, generated from the provider schema by
`kgen module`. Do not edit by hand -- regenerate instead.

## Usage

```hcl
module "billing_rule" {
  source = "..."

  billing_source_ids = []
  description        = "example"
  name               = "example"
  rule_type          = 1
  rule_value         = 1
  start_month        = 1
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
| [kion_billing_rule.this](https://registry.terraform.io/providers/kionsoftware/kion/1.0.0/docs/resources/billing_rule) | resource |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| billing\_source\_ids | List of Billing Source IDs attached to the billing rule. | `set(number)` | n/a | yes |
| description | Description of the billing rule. The description can be up to 1024 characters long. | `string` | n/a | yes |
| name | Name of the billing rule. The name can be up to 255 characters long. | `string` | n/a | yes |
| rule\_type | Type of the billing rule. The allowed values are: 1 - Rate Conversion 2 - Markup 3 - Discount | `number` | n/a | yes |
| rule\_value | The Value of the Billing Rule. This is handled differently based on the Type of Billing Rules. For instance: For a 5% markup, the value would be 5.0 For a 10% discount, the value would be 10.0 For a 1.214 currency conversion, the value would be 1.214 | `number` | n/a | yes |
| start\_month | Start month of the billing rule. This must be in the format YYYYMM where 202501 represents January 2025. | `number` | n/a | yes |
| end\_month | End month of the billing rule. This must be in the format YYYYMM where 202501 represents January 2025. The end month is exclusive, e.g. if you want the rule to end in January 2025, send a value of 202502. Omit if the end month should be "Never Ends". If included, the value must be after the start month. | `number` | `null` | no |

## Outputs

| Name | Description |
| ---- | ----------- |
| id | id of the kion\_billing\_rule. |
<!-- END_TF_DOCS -->
