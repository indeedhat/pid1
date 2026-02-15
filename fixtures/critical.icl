version = 1

service "non-critical" {
  command = "./fixtures/service_critical_aditional.sh"
  critical = true
}

# vi: ft=hcl
