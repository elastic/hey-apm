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
    string(name: 'ELASTIC_STACK_VERSION', defaultValue: "6.4", description: "Elastic Stack Git branch/tag to use")
    string(name: 'APM_SERVER_VERSION', defaultValue: "6.4", description: "APM Server Git branch/tag to use")
    
    /*
    string(name: 'hey_e', defaultValue: "3", description: "number of errors (default 3)")
    string(name: 'hey_f', defaultValue: "20", description: "number of stacktrace frames per span (default 20)")
    string(name: 'hey_idle_timeout', defaultValue: "3", description: "idle timeout (default 3m0s)")
    string(name: 'hey_method', defaultValue: "POST", description: "method type (default 'POST')")
    string(name: 'hey_p', defaultValue: "1", description: "Only used if qps is not set. Defines the pause between sending events over the same http request. (default 1ms)")
    string(name: 'hey_q', defaultValue: "5", description: "queries per second")
    string(name: 'hey_request-timeout', defaultValue: "10", description: "request timeout in seconds (default 10s)")
    string(name: 'hey_requests', defaultValue: "2147483647", description: "maximum requests to make (default 2147483647)")
    string(name: 'hey_run', defaultValue: "30", description: "stop run after this duration (default 30s)")
    booleanParam(name: 'hey_stream', defaultValue: false, description: "send data in a streaming way via http")
    string(name: 'hey_s', defaultValue: "7", description: "number of spans (default 7)")
    string(name: 'hey_t', defaultValue: "6", description: "number of transactions (default 6)")
    string(name: 'hey_header', defaultValue: "", description: "header(s) added to all requests")
    booleanParam(name: 'hey_disable_compression', defaultValue: false, description: "Disable compression")
    booleanParam(name: 'hey_disable_keepalive', defaultValue: false, description: "Disable keepalive")
    booleanParam(name: 'hey_disable_redirects', defaultValue: false, description: "Disable redirects")
    */
    booleanParam(name: 'hey_apm_ci', defaultValue: true, description: 'Enable run integration test')
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
      when { 
        beforeAgent true
        environment name: 'hey_apm_ci', value: 'true' 
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
    }
    stage('Integration Tests') {
      failFast true
      parallel {
        /**
          Unit tests and apm-server stress testing.
        */
        stage('Hey APM test') { 
          agent { label 'linux && immutable' }
          when { 
            beforeAgent true
            environment name: 'hey_apm_ci', value: 'true' 
          }
          environment {
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
              coverageReport("${BASE_DIR}/build/coverage")
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
      echo 'Post Actions'
    }
    success { 
      echo 'Success Post Actions'
    }
    aborted { 
      echo 'Aborted Post Actions'
    }
    failure { 
      echo 'Failure Post Actions'
      //step([$class: 'Mailer', notifyEveryUnstableBuild: true, recipients: "${NOTIFY_TO}", sendToIndividuals: false])
    }
    unstable { 
      echo 'Unstable Post Actions'
    }
  }
}