def lastStage = ''
node('node') {
  properties([disableConcurrentBuilds()])
    currentBuild.result = "SUCCESS"

    stage('Checkout') {
      lastStage = env.STAGE_NAME
      checkout scm

      echo "Current build result: ${currentBuild.result}"
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
        archiveArtifacts artifacts: 'release/**/storagenode*.msi', followSymlinks: false
        archiveArtifacts artifacts: 'release/**/storagenode*.exe', followSymlinks: false
        echo "Current build result: ${currentBuild.result}"
      }
    }

    stage('Sign Windows Installer') {
      lastStage = env.STAGE_NAME
      unstash "storagenode-installer"

      sh 'make sign-windows-installer'

      echo "Current build result: ${currentBuild.result}"
    }

}
