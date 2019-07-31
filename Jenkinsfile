#!/usr/bin/env groovy

@Library('apm@current') _

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
    BENCHMARK_SECRET  = 'secret/apm-team/ci/apm-server-benchmark-cloud'
  }
  options {
    timeout(time: 1, unit: 'HOURS')
    buildDiscarder(logRotator(numToKeepStr: '20', artifactNumToKeepStr: '20', daysToKeepStr: '30'))
    timestamps()
    ansiColor('xterm')
    disableResume()
    durabilityHint('PERFORMANCE_OPTIMIZED')
    rateLimitBuilds(throttle: [count: 60, durationName: 'hour', userBoost: true])
    quietPeriod(10)
  }
  triggers {
    cron('H H(3-5) * * 1-5')
  }
  parameters {
    string(name: 'GO_VERSION', defaultValue: "1.12.1", description: "Go version to use.")
    string(name: 'STACK_VERSION', defaultValue: "7.3.0-SNAPSHOT", description: "Stack version Git branch/tag to use")
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
            withGithubNotify(context: 'Test', tab: 'tests') {
              deleteDir()
              unstash 'source'
              dir("${BASE_DIR}"){
                sh "./scripts/jenkins/unit-test.sh ${GO_VERSION}"
              }
            }
          }
          post {
            always {
              coverageReport("${BASE_DIR}/build/coverage")
              junit(allowEmptyResults: true,
                keepLongStdio: true,
                testResults: "${BASE_DIR}/build/*.xml")
            }
          }
        }
        /**
          APM server benchmark.
        */
        stage('Benchmark') {
          agent { label 'metal' }
          steps {
            withGithubNotify(context: 'Benchmark', tab: 'tests') {
              deleteDir()
              unstash 'source'
              script {
                dir(BASE_DIR){
                  sendBenchmarks.prepareAndRun(secret: env.BENCHMARK_SECRET, url_var: 'ES_URL',
                                               user_var: 'ES_USER', pass_var: 'ES_PASS') {
                    sh 'scripts/jenkins/run-bench-in-docker.sh'
                  }
                }
              }
            }
          }
        }
      }
    }
  }
  post {
    always {
      notifyBuildResult()
    }
  }
}
