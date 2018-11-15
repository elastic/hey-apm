#!/usr/bin/env groovy

library identifier: 'apm@master',
changelog: false,
retriever: modernSCM(
  [$class: 'GitSCMSource', 
  credentialsId: 'f6c7695a-671e-4f4f-a331-acdce44ff9ba', 
  remote: 'git@github.com:elastic/apm-pipeline-library.git'])

pipeline {
  agent any
  environment {
    HOME = "${env.HUDSON_HOME}"
    BASE_DIR="src/github.com/elastic/hey-apm"
    APM_SERVER_BASE_DIR = "src/github.com/elastic/apm-server"
    JOB_GIT_CREDENTIALS = "f6c7695a-671e-4f4f-a331-acdce44ff9ba"
  }
  triggers {
    cron('0 0 * * 1-5')
  }
  options {
    timeout(time: 1, unit: 'HOURS') 
    buildDiscarder(logRotator(numToKeepStr: '3', artifactNumToKeepStr: '2', daysToKeepStr: '30'))
    timestamps()
    preserveStashes()
    //see https://issues.jenkins-ci.org/browse/JENKINS-11752, https://issues.jenkins-ci.org/browse/JENKINS-39536, https://issues.jenkins-ci.org/browse/JENKINS-54133 and jenkinsci/ansicolor-plugin#132
    //ansiColor('xterm')
    disableResume()
    durabilityHint('PERFORMANCE_OPTIMIZED')
  }
  parameters {
    string(name: 'branch_specifier', defaultValue: "", description: "the Git branch specifier to build (branchName, tagName, commitId, etc.)")
    string(name: 'GO_VERSION', defaultValue: "1.10.3", description: "Go version to use.")
    string(name: 'APM_SERVER_VERSION', defaultValue: "6.4", description: "APM Server Git branch/tag to use")
  }
  stages {
    /**
     Checkout the code and stash it, to use it on other stages.
    */
    stage('Checkout') {
      agent { label 'master || linux' }
      environment {
        PATH = "${env.PATH}:${env.HUDSON_HOME}/go/bin/:${env.WORKSPACE}/bin"
        GOPATH = "${env.WORKSPACE}"
      }
      steps {
          withEnvWrapper() {
              dir("${BASE_DIR}"){
                script{
                  if(!env?.branch_specifier){
                    echo "Checkout SCM"
                    checkout scm
                  } else {
                    echo "Checkout ${branch_specifier}"
                    checkout([$class: 'GitSCM', branches: [[name: "${branch_specifier}"]], 
                      doGenerateSubmoduleConfigurations: false, 
                      extensions: [], 
                      submoduleCfg: [], 
                      userRemoteConfigs: [[credentialsId: "${JOB_GIT_CREDENTIALS}", 
                      url: "${GIT_URL}"]]])
                  }
                  env.JOB_GIT_COMMIT = getGitCommitSha()
                  env.JOB_GIT_URL = "${GIT_URL}"
                  
                  github_enterprise_constructor()
                  
                  on_change{
                    echo "build cause a change (commit or PR)"
                  }
                  
                  on_commit {
                    echo "build cause a commit"
                  }
                  
                  on_merge {
                    echo "build cause a merge"
                  }
                  
                  on_pull_request {
                    echo "build cause PR"
                  }
                }
              }
              stash allowEmpty: true, name: 'source', useDefaultExcludes: false
          }
      }
    }
    /**
      Unit tests.
    */
    stage('Test') { 
      agent { label 'linux && immutable' }
      environment {
        PATH = "${env.PATH}:${env.HUDSON_HOME}/go/bin/:${env.WORKSPACE}/bin"
        GOPATH = "${env.WORKSPACE}"
      }
      steps {
        withEnvWrapper() {
          unstash 'source'
          dir("${BASE_DIR}"){
            sh """#!/bin/bash
            ./scripts/jenkins/unit-test.sh
            """
          }
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
      agent { label 'linux && immutable' }
      environment {
        PATH = "${env.PATH}:${env.HUDSON_HOME}/go/bin/:${env.WORKSPACE}/bin"
        GOPATH = "${env.WORKSPACE}"
        APM_SERVER_DIR = "${env.GOPATH}/${env.APM_SERVER_BASE_DIR}"
      }
      steps {
        withEnvWrapper() {
          unstash 'source'
          dir("${APM_SERVER_BASE_DIR}"){
            checkout([$class: 'GitSCM', branches: [[name: "${APM_SERVER_VERSION}"]], 
              doGenerateSubmoduleConfigurations: false, 
              extensions: [], 
              submoduleCfg: [], 
              userRemoteConfigs: [[credentialsId: "${JOB_GIT_CREDENTIALS}", 
              url: "https://github.com/elastic/apm-server.git"]]])
          }
          dir("${BASE_DIR}"){
            withEsEnv(secret: 'apm-server-benchmark-cloud'){
              sh """#!/bin/bash
              ./scripts/jenkins/run-test.sh
              """
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
  post {
    success {
      echoColor(text: '[SUCCESS]', colorfg: 'green', colorbg: 'default')
    }
    aborted {
      echoColor(text: '[ABORTED]', colorfg: 'magenta', colorbg: 'default')
    }
    failure { 
      echoColor(text: '[FAILURE]', colorfg: 'red', colorbg: 'default')
      //step([$class: 'Mailer', notifyEveryUnstableBuild: true, recipients: "${NOTIFY_TO}", sendToIndividuals: false])
    }
    unstable { 
      echoColor(text: '[UNSTABLE]', colorfg: 'yellow', colorbg: 'default')
    }
  }
}