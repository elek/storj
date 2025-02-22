def lastStage = ''
node('node') {
  properties([disableConcurrentBuilds()])
  try {
    currentBuild.result = "SUCCESS"

    stage('Checkout') {
      lastStage = env.STAGE_NAME
      checkout scm

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Run Rolling Upgrade Test') {
        lastStage = env.STAGE_NAME
        try {
          echo "Running Rolling Upgrade test"

          env.STORJ_SIM_POSTGRES = 'postgres://postgres@postgres:5432/teststorj?sslmode=disable'
          env.STORJ_SIM_REDIS = 'redis:6379'
          env.STORJ_MIGRATION_DB = 'postgres://postgres@postgres:5432/teststorj?sslmode=disable&options=--search_path=satellite/0/meta'

          echo "STORJ_SIM_POSTGRES: $STORJ_SIM_POSTGRES"
          echo "STORJ_SIM_REDIS: $STORJ_SIM_REDIS"
          echo "STORJ_MIGRATION_DB: $STORJ_MIGRATION_DB"
          sh 'docker run --rm -d -e POSTGRES_HOST_AUTH_METHOD=trust --name postgres-$BUILD_NUMBER postgres:12.3'
          sh 'docker run --rm -d --name redis-$BUILD_NUMBER redis:latest'

          sh '''until $(docker logs postgres-$BUILD_NUMBER | grep "database system is ready to accept connections" > /dev/null)
                do printf '.'
                sleep 5
                done
            '''
          sh 'docker exec postgres-$BUILD_NUMBER createdb -U postgres teststorj'
          // fetch the remote main branch
          sh 'git fetch --no-tags --progress -- https://github.com/storj/storj.git +refs/heads/main:refs/remotes/origin/main'
          sh 'docker run -u $(id -u):$(id -g) --rm -i -v $PWD:$PWD -w $PWD --entrypoint $PWD/scripts/tests/rollingupgrade/test-sim-rolling-upgrade.sh -e BRANCH_NAME -e STORJ_SIM_POSTGRES -e STORJ_SIM_REDIS -e STORJ_MIGRATION_DB --link redis-$BUILD_NUMBER:redis --link postgres-$BUILD_NUMBER:postgres storjlabs/golang:1.21.3'
        }
        catch(err){
            throw err
        }
        finally {
          sh 'docker stop postgres-$BUILD_NUMBER || true'
          sh 'docker rm postgres-$BUILD_NUMBER || true'
          sh 'docker stop redis-$BUILD_NUMBER || true'
          sh 'docker rm redis-$BUILD_NUMBER || true'
        }
    }

    stage('Build Binaries') {
      lastStage = env.STAGE_NAME
      sh 'make binaries'

      stash name: "storagenode-binaries", includes: "release/**/storagenode*.exe"

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Build Windows Installer') {
      lastStage = env.STAGE_NAME
      node('windows') {
        checkout scm

        unstash "storagenode-binaries"

        bat 'installer\\windows\\buildrelease.bat'

        stash name: "storagenode-installer", includes: "release/**/storagenode*.msi"

        echo "Current build result: ${currentBuild.result}"
      }
    }

    stage('Sign Windows Installer') {
      lastStage = env.STAGE_NAME
      unstash "storagenode-installer"

      sh 'make sign-windows-installer'

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Build Images') {
      lastStage = env.STAGE_NAME
      sh 'make images'

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Push Images') {
      lastStage = env.STAGE_NAME
      sh 'make push-images'

      echo "Current build result: ${currentBuild.result}"
    }

    stage('Upload') {
      lastStage = env.STAGE_NAME
      sh 'make binaries-upload'

      echo "Current build result: ${currentBuild.result}"
    }
    stage('Draft Release') {
      withCredentials([string(credentialsId: 'GITHUB_RELEASE_TOKEN', variable: 'GITHUB_TOKEN')]) {
        lastStage = env.STAGE_NAME
        sh 'make draft-release'

        echo "Current build result: ${currentBuild.result}"
      }
    }

  }
  catch (err) {
    echo "Caught errors! ${err}"
    echo "Setting build result to FAILURE"
    currentBuild.result = "FAILURE"

    slackSend color: 'danger', message: "@build-team ${env.BRANCH_NAME} build failed during stage ${lastStage} ${env.BUILD_URL}"

    throw err

  }
  finally {
    stage('Cleanup') {
      sh 'make clean-images'
      deleteDir()
    }

  }
}
