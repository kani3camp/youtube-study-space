output "policy_names" {
  description = "Policy IDs for future configuration-driven import addressing."
  value       = { for key, policy in google_monitoring_alert_policy.quota : key => policy.name }
}
