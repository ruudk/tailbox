version = 1

defaults {
  lines = 6
  stop_signal = "int"
  stop_timeout = 10
}

workflow Setup  {
  run Tail {
    background = true
    command = "tail -f error"
  }

  run "Homebrew dependencies" {
    stop_timeout = 0
    command = <<-SHELL
      ./brew-bundle
      SHELL
  }

  watcher "Watching Composer changes" {
    glob = ["composer.lock"]

    run "Installing Composer dependencies" {
      command = "composer install --no-interaction"
    }

    run "Restart workers" {
      command = "supervisorctl restart resque kafka-consumer"
    }
  }

  wait "Wait for Docker to be ready" {
    interval = 5
    timeout = 300
    url = "http://127.0.0.1:123"
  }

  run "Reset Kafka offsets" {
    command = <<-SHELL
      ./brew-bundle
      SHELL

  }

  run "Installing Composer dependencies" {
    command = "./one-twente-wait"
  }
}

workflow Run {
  watcher "Watching Composer changes" {
    glob = ["composer.lock"]

    run "Installing Composer dependencies" {
      command = "composer install --no-interaction"
    }

    run "Restart workers" {
      command = "supervisorctl restart resque kafka-consumer"
    }
  }

  run "Tail application log" {
    command = "tail -f /Volumes/CS/www/website/var/log/symfony.dev.log"
    lines = 15
  }
}

workflow Shutdown {
  when = "shutdown"

  run "Stopping Supervisor" {
    when = "always"
    command = <<-SHELL
      #./bin-dev/supervisorctl stop all
      #./bin-dev/supervisorctl shutdown
      echo "Stopping supervisor.."
      sleep 1
      SHELL
  }

  run "Stopping Symfony Webserver" {
    when = "always"
    command = "./one-twente-wait"
  }

  run "Stopping Docker" {
    when = "always"
    command = <<-SHELL
      echo stopping docker
      sleep 1
      echo docker stopped
      SHELL
  }
}
