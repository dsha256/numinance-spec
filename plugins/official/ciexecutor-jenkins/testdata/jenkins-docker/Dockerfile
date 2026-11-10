# Throwaway Jenkins controller for provider/live_docker_test.go. Not for any use outside that test.
FROM jenkins/jenkins:lts-jdk21@sha256:74401de6a6f92e1afe1b02d1c8e3d0c33f263bfb5275dd8813f9150355f2367b

ENV JAVA_OPTS="-Djenkins.install.runSetupWizard=false"
RUN jenkins-plugin-cli --plugins cloudbees-folder
COPY init.groovy.d/10-fixture.groovy /usr/share/jenkins/ref/init.groovy.d/10-fixture.groovy
