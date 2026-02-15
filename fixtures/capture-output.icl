version = 1

service "non-prefixed" {
  command = "./fixtures/service_output_aditional.sh"
  args = ["non-prefixed"]
  capture_output = true
}

service "prefixed" {
  command = "./fixtures/service_output_aditional.sh"
  args = ["prefixed"]
  capture_output = true
  capture_prefix = true
}

# vi: ft=hcl
