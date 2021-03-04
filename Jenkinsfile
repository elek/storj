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
    if (env.BRANCH_NAME == "main") {
        stage('Run Versions Test') {
            lastStage = env.STAGE_NAME
            try {
              echo "Running Versions test"

              env.STORJ_SIM_POSTGRES = 'postgres://postgres@postgres:5432/teststorj?sslmode=disable'
              env.STORJ_SIM_REDIS = 'redis:6379'

              echo "STORJ_SIM_POSTGRES: $STORJ_SIM_POSTGRES"
              echo "STORJ_SIM_REDIS: $STORJ_SIM_REDIS"
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
              sh 'docker run -u $(id -u):$(id -g) --rm -i -v $PWD:$PWD -w $PWD --entrypoint $PWD/scripts/tests/testversions/test-sim-versions.sh -e STORJ_SIM_POSTGRES -e STORJ_SIM_REDIS --link redis-$BUILD_NUMBER:redis --link postgres-$BUILD_NUMBER:postgres -e CC=gcc storjlabs/golang:1.15.7'
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
    }

    stage('Build Binaries') {
      lastStage = env.STAGE_NAME
      sh 'make binaries'

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

  }
  catch (err) {
    echo "Caught errors! ${err}"
    echo "Setting build result to FAILURE"
    currentBuild.result = "FAILURE"

    slackSend color: 'danger', message: "@build-team ${env.BRANCH_NAME} build failed during stage ${lastStage} ${env.BUILD_URL}"

    mail from: 'builds@storj.io',
      replyTo: 'builds@storj.io',
      to: 'builds@storj.io',
      subject: "storj/storj branch ${env.BRANCH_NAME} build failed",
      body: "Project build log: ${env.BUILD_URL}"

      throw err

  }
  finally {
    stage('Cleanup') {
      sh 'make clean-images'
      deleteDir()
    }

  }
}
