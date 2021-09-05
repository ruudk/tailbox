version = 1

defaults {
  lines = 6
  stop_signal = "int"
  stop_timeout = 10
}

workflow "Setup"  {
  wait "Wait for file" {
    interval = 5
    timeout = 300
    url = "file://${directory}/ready"
  }

  wait "Wait for HTTP" {
    interval = 5
    timeout = 300
    url = "https://www.google.com"
  }

  wait "Wait for tcp port" {
    interval = 5
    timeout = 300
    url = "tcp://127.0.0.1:123"
  }
}
