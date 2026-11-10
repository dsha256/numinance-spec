// Fixture for provider/live_docker_test.go: a Jenkins with one admin, crumbs on, no anonymous read, and the folder layout the
// ciexecutor-jenkins provider expects (acme/widgets/<pipeline>). The credentials come from the environment of the container,
// generated at random by the test for each run; nothing secret is in this file.
import com.cloudbees.hudson.plugins.folder.Folder
import hudson.model.FreeStyleProject
import hudson.model.ParametersDefinitionProperty
import hudson.model.StringParameterDefinition
import hudson.model.User
import hudson.security.FullControlOnceLoggedInAuthorizationStrategy
import hudson.security.HudsonPrivateSecurityRealm
import hudson.security.csrf.DefaultCrumbIssuer
import hudson.tasks.Shell
import jenkins.model.Jenkins
import jenkins.security.ApiTokenProperty

def jenkins = Jenkins.get()
def adminUser = System.getenv('NUMINANCE_TEST_ADMIN_USER')
def adminToken = System.getenv('NUMINANCE_TEST_ADMIN_TOKEN')
if (!adminUser || !adminToken) {
    throw new IllegalStateException('NUMINANCE_TEST_ADMIN_USER and NUMINANCE_TEST_ADMIN_TOKEN must be set')
}

def realm = new HudsonPrivateSecurityRealm(false)
// The account password is the token too; the provider's requests authenticate with the API token below, so that a crumb is not
// bound to a session (the provider keeps no cookies).
realm.createAccount(adminUser, adminToken)
jenkins.setSecurityRealm(realm)

def strategy = new FullControlOnceLoggedInAuthorizationStrategy()
strategy.setAllowAnonymousRead(false)
jenkins.setAuthorizationStrategy(strategy)
jenkins.setCrumbIssuer(new DefaultCrumbIssuer(false))

// A fixed API token: the plain value is "11" followed by 32 hex characters, which is how Jenkins writes its own.
def user = User.get(adminUser)
def property = user.getProperty(ApiTokenProperty)
property.tokenStore.addFixedNewToken('numinance-test', adminToken)
user.save()

def folder = jenkins.getItem('acme') ?: jenkins.createProject(Folder, 'acme')
def widgets = folder.getItem('widgets') ?: folder.createProject(Folder, 'widgets')

['release', 'dry'].each { name ->
    def job = widgets.getItem(name) ?: widgets.createProject(FreeStyleProject, name)
    job.addProperty(new ParametersDefinitionProperty(
        new StringParameterDefinition('numinance_revision', ''),
        new StringParameterDefinition('numinance_operation_id', ''),
        new StringParameterDefinition('environment', 'staging')))
    // A started build stays running until it is stopped.
    job.buildersList.add(new Shell('sleep 300'))
    job.save()
}

jenkins.save()
