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
    string(name: 'APM_SERVER_VERSION', defaultValue: "7.3.0-SNAPSHOT", description: "APM Server Git branch/tag to use")
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
              dir("${BASE_DIR}"){
                sendBenchmark(env.BENCHMARK_SECRET) {
                  sh 'scripts/jenkins/run-bench-in-docker.sh'
                }
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
    always {
      notifyBuildResult()
    }
  }
}

// TODO: move to the shared library
def sendBenchmark(String secretPath, Closure body) {
  def props = getVaultSecret(secret: secretPath)
  if(props?.errors){
     error "sendBenchmark: Unable to get credentials from the vault: " + props.errors.toString()
  }

  def data = props?.data
  def url = data?.url
  def user = data?.user
  def password = data?.password

  if(data == null || user == null || password == null || url == null){
    error 'sendBenchmark: was not possible to get authentication info to send benchmarks'
  }

  def protocol = getProtocol(url)

  log(level: 'INFO', text: 'sendBenchmark: run script...')
  wrap([$class: 'MaskPasswordsBuildWrapper', varPasswordPairs: [
    [var: 'ES_URL', password: "${protocol}${url}"],
    [var: 'ES_USER', password: "${user}"],
    [var: 'ES_PASS', password: "${password}"]
    ]]) {
    withEnv(["ES_URL=${protocol}${url}", "ES_USER=${user}", "ES_PASS=${password}"]){
      body()
    }
  }
}

def getProtocol(url){
  def protocol = 'https://'
  if(url.startsWith('https://')){
    protocol = 'https://'
  } else if (url.startsWith('http://')){
    log(level: 'INFO', text: "sendBenchmark: you are using 'http' protocol to access to the service.")
    protocol = 'http://'
  } else {
    error 'sendBenchmark: unknow protocol, the url is not http(s).'
  }
  return protocol
}
