#!/usr/bin/env groovy

pipeline {
  agent any
  environment {
    BASE_DIR="src/github.com/elastic/hey-apm"
    APM_SERVER_BASE_DIR = "src/github.com/elastic/apm-server"
    JOB_GIT_CREDENTIALS = "f6c7695a-671e-4f4f-a331-acdce44ff9ba"
    GO_VERSION = "${params.GO_VERSION}"
    APM_SERVER_VERSION = "${params.APM_SERVER_VERSION}"
    NOTIFY_TO = credentials('notify-to')
    JOB_GCS_BUCKET = credentials('gcs-bucket')
  }
  options {
    timeout(time: 1, unit: 'HOURS') 
    buildDiscarder(logRotator(numToKeepStr: '20', artifactNumToKeepStr: '20', daysToKeepStr: '30'))
    timestamps()
    ansiColor('xterm')
    disableResume()
    durabilityHint('PERFORMANCE_OPTIMIZED')
  }
  parameters {
    string(name: 'GO_VERSION', defaultValue: "1.11.2", description: "Go version to use.")
    string(name: 'APM_SERVER_VERSION', defaultValue: "6.4", description: "APM Server Git branch/tag to use")
  }
  stages {
    stage('Initializing'){
      agent { label 'linux && immutable' }
      options { skipDefaultCheckout() }
      environment {
        PATH = "${env.PATH}:${env.WORKSPACE}/bin"
        HOME = "${env.WORKSPACE}"
        GOPATH = "${env.WORKSPACE}"
      }
      stages {
        /**
         Checkout the code and stash it, to use it on other stages.
        */
        stage('Checkout') {
          steps {
            deleteDir()
            gitCheckout(basedir: "${BASE_DIR}")
            stash allowEmpty: true, name: 'source', useDefaultExcludes: false
          }
        }
        /**
          Unit tests.
        */
        stage('Test') { 
          steps {
            deleteDir()
            unstash 'source'
            dir("${BASE_DIR}"){
              sh './scripts/jenkins/unit-test.sh'
            }
          }
          post {
            always {
              coverageReport("${BASE_DIR}/build/coverage")
              junit(allowEmptyResults: true, 
                keepLongStdio: true, 
                testResults: "${BASE_DIR}/build/junit-report.xml,${BASE_DIR}/build/TEST-*.xml")
            }
          }
        }
        /**
          APM server stress tests.
        */
        stage('Hey APM test') {
          environment {
            APM_SERVER_DIR = "${env.WORKSPACE}/${env.APM_SERVER_BASE_DIR}"
          }
          steps {
            deleteDir()
            unstash 'source'
            dir("${APM_SERVER_BASE_DIR}"){
              checkout([$class: 'GitSCM', branches: [[name: "${APM_SERVER_VERSION}"]], 
                doGenerateSubmoduleConfigurations: false, 
                extensions: [], 
                submoduleCfg: [], 
                userRemoteConfigs: [[credentialsId: "${JOB_GIT_CREDENTIALS}", 
                url: "git@github.com:elastic/apm-server.git"]]])
            }
            dir("${BASE_DIR}"){
              withEsEnv(secret: 'apm-server-benchmark-cloud'){
                sh './scripts/jenkins/run-test.sh'
              }
            }
          }
          post {
            always {
              junit(allowEmptyResults: true,
                keepLongStdio: true,
                testResults: "${BASE_DIR}/build/junit-*.xml,${BASE_DIR}/build/TEST-*.xml")
            }
          }
        }
      }
    }
  }
  post {
    success {
      echoColor(text: '[SUCCESS]', colorfg: 'green', colorbg: 'default')
    }
    aborted {
      echoColor(text: '[ABORTED]', colorfg: 'magenta', colorbg: 'default')
    }
    failure { 
      echoColor(text: '[FAILURE]', colorfg: 'red', colorbg: 'default')
      step([$class: 'Mailer', notifyEveryUnstableBuild: true, recipients: "${NOTIFY_TO}", sendToIndividuals: false])
    }
    unstable { 
      echoColor(text: '[UNSTABLE]', colorfg: 'yellow', colorbg: 'default')
      step([$class: 'Mailer', notifyEveryUnstableBuild: true, recipients: "${NOTIFY_TO}", sendToIndividuals: false])
    }
  }
}
