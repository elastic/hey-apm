// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

@Library('apm@current') _

pipeline {
  agent any
  environment {
    BASE_DIR = 'src/github.com/elastic/hey-apm'
    VERSION_FILE = 'https://raw.githubusercontent.com/elastic/apm-server/master/vendor/github.com/elastic/beats/libbeat/version/version.go'
    JOB_GIT_CREDENTIALS = "f6c7695a-671e-4f4f-a331-acdce44ff9ba"
    GO_VERSION = "${params.GO_VERSION}"
    STACK_VERSION = "${params.STACK_VERSION}"
    NOTIFY_TO = credentials('notify-to')
    JOB_GCS_BUCKET = credentials('gcs-bucket')
    BENCHMARK_SECRET  = 'secret/apm-team/ci/java-agent-benchmark-cloud'
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
    cron('H H(3-5) * * 1-5')
  }
  parameters {
    string(name: 'GO_VERSION', defaultValue: '1.12.1', description: 'Go version to use.')
    string(name: 'STACK_VERSION', defaultValue: '', description: 'Stack version Git branch/tag to use. Default behavior uses the apm-server@master version.')
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
            script {
              if (params.STACK_VERSION.trim()) {
                env.STACK_VERSION = getVersion()
              }
            }
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
          post {
            always {
              archiveArtifacts "${BASE_DIR}/build/environment.txt"
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

def getVersion() {
  return sh(script: """curl -s ${VERSION_FILE}  | grep defaultBeatVersion | cut -d'=' -f2 | sed 's#"##g'""", returnStdout: true).trim()
}
