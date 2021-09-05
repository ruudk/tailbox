version = 1

defaults {
  lines = 6
  stop_signal = "int"
  stop_timeout = 10
}

workflow main {
  watcher "Watching Composer changes" {
    glob {
      directory = "${directory}"
      pattern = "composer.lock"
    }

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
