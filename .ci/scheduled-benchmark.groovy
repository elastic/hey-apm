#!/usr/bin/env groovy

@Library('apm@current') _

pipeline {
  agent { label 'linux && immutable' }
  environment {
    BASE_DIR = 'src/github.com/elastic/hey-apm'
    JOB_GIT_CREDENTIALS = "f6c7695a-671e-4f4f-a331-acdce44ff9ba"
    GO_VERSION = "${params.GO_VERSION}"
    STACK_VERSION = "${params.STACK_VERSION}"
    APM_DOCKER_IMAGE = "${params.APM_DOCKER_IMAGE}"
    NOTIFY_TO = credentials('notify-to')
    JOB_GCS_BUCKET = credentials('gcs-bucket')
    BENCHMARK_SECRET  = 'secret/apm-team/ci/benchmark-cloud'
    DOCKER_SECRET = 'secret/apm-team/ci/docker-registry/prod'
    DOCKER_REGISTRY = 'docker.elastic.co'
  }
  options {
    timeout(time: 1, unit: 'HOURS')
    buildDiscarder(logRotator(numToKeepStr: '20', artifactNumToKeepStr: '20', daysToKeepStr: '30'))
    timestamps()
    ansiColor('xterm')
    disableResume()
    durabilityHint('PERFORMANCE_OPTIMIZED')
  }
  triggers {
    cron('H H(3-5) * * *')
  }
  parameters {
    string(name: 'GO_VERSION', defaultValue: '1.14', description: 'Go version to use.')
    string(name: 'STACK_VERSION', defaultValue: '8.0.0-SNAPSHOT', description: 'Stack version Git branch/tag to use.')
    string(name: 'APM_DOCKER_IMAGE', defaultValue: 'docker.elastic.co/apm/apm-server', description: 'The docker image to be used.')
  }
  stages {
    stage('Initializing'){
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
            gitCheckout(basedir: env.BASE_DIR, repo: 'git@github.com:elastic/hey-apm.git',
                        branch: 'main', credentialsId: env.JOB_GIT_CREDENTIALS)
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
              sh "./.ci/scripts/unit-test.sh ${GO_VERSION}"
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
            deleteDir()
            unstash 'source'
            dockerLogin(secret: env.DOCKER_SECRET, registry: env.DOCKER_REGISTRY)
            script {
              dir(BASE_DIR){
                sendBenchmarks.prepareAndRun(secret: env.BENCHMARK_SECRET, url_var: 'ES_URL',
                                             user_var: 'ES_USER', pass_var: 'ES_PASS') {
                  sh '.ci/scripts/run-bench-in-docker.sh'
                }
              }
            }
          }
          post {
            always {
              archiveArtifacts "${BASE_DIR}/build/environment.txt"
              deleteDir()
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
