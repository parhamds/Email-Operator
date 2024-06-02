# Email Operator Documentation

## Table of Contents

1. [Introduction](#introduction)
2. [Prerequisites](#prerequisites)
3. [Installation](#installation)
4. [Deployment](#deployment)
   - [Quick Deploy](#quick-deploy)
   - [Deploy on Cluster](#deploy-on-cluster)
5. [Usage](#usage)
   - [Creating APITokenSecret](#creating-apitokensecret)
   - [Creating EmailSenderConfig](#creating-emailsenderconfig)
   - [Creating Email](#creating-email)
6. [Test the Operator](#test-the-operator)
7. [Updating the Operator](#updating-the-operator)
8. [Contributing](#contributing)
9. [Contact](#contact)

## Introduction

The Email Operator is a Kubernetes custom controller that manages the lifecycle of email sending configurations and email sending tasks within a Kubernetes cluster. It uses custom resources `EmailSenderConfig` and `Email` to define email sending configurations and tasks respectively.

In this implementation i used `Kind` as a test kubernetes cluster and `KubeBuilder` to generate Kubernetes APIs.

## Prerequisites

- Kubernetes cluster (v1.11.3+)
- kubectl (v1.11.3+)
- Go (v1.20.0+)
- Doker (v17.03+.)
- kubebuilder

## Installation

To install the CRDs, you need to follow these steps:

1. Clone the repository:

   ```sh
   git clone https://github.com/parhamds/Email-Operator.git
   cd Email-Operator
   ```

2. Install the CRDs (Custom Resource Definitions):

   ```sh
   make install
   ```

## Deployment

### Quick Deploy

For quick feedback and code-level debugging, run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

   ```sh
   make run
   ```

### Deploy on Cluster
When your controller is ready to be packaged and tested in other clusters, build and push your image to the location specified by IMG:

```sh
make docker-build docker-push IMG=<some-registry>/<project-name>:tag
```
```sh
make deploy IMG=<some-registry>/<project-name>:tag
```
Currently, the latest version is uploaded on `parhamds/email:latest`

## Usage

### Creating APITokenSecret

`EmailSenderConfig` defines a k8s secret that contains the API token for the mailer provider.

Example YAML for `apiTokenSecret`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret_name>
type: Opaque
data:
  apiToken: <base64_encoded_api_token>
```

### Creating EmailSenderConfig

`EmailSenderConfig` defines the configuration for sending emails. It includes information like the mailer provider ("MailSender" or "MailGun") sender's email address and the name of the secret resource which contains the API token required for authentication.

Example YAML for `EmailSenderConfig`:

```yaml
metadata:
  labels:
    app.kubernetes.io/name: email-v1
    app.kubernetes.io/managed-by: kustomize
  name: <emailsenderconfig_name>
spec:
  provider: <provider>
  apiTokenSecretRef: <name_of_api_token_secret>
  senderEmail: <sender_email>
```

### Creating Email

`Email` defines the email sending task. It includes information like the recipient's email address, subject, and body of the email.

Example YAML for `Email`:

```yaml
apiVersion: parham.my.domain/v1
kind: Email
metadata:
  name: <email_name>
  namespace: default
spec:
  senderConfigRef: <name_of_emailsenderconfig>
  recipientEmail: <recipient_email>
  subject: <email_subject>
  body: <email_body>
```

To create resources, apply the YAML files using:

```sh
kubectl apply -f <resource_yaml_file>
```
You can find resource samples in the `/samples/` folder of this repo.

## Test the Operator
1. Create an env file named "env" in the root folder of the cloned repo, like the file below. It should contain 2 valid data (1 from MailSender and 1 from MailGun) and 1 invalid data to test if invalid data is handled properly:
```sh
# .env
API_TOKEN_MAILERSEND=<your_mailersend_api_token>
SENDER_MAIL_MAILERSEND=<your_mailersend_sender_email_address>
API_TOKEN_MAILGUN=<your_mailgun_api_token>
SENDER_MAIL_MAILGUN=<your_mailgun_sender_email_address>
API_TOKEN_INVALID=<an_invalid_api_token>
SENDER_MAIL_INVALID=<an_invalid_sender_email_address>
```
2. Update the recipientEmail address in `/internal/controller/testdata.json` to your desired recipient email address to receive test emails.

3. Run the test

```sh
cd internal/controller/
ginkgo --json-report=report.json --junit-report=report.xml
```
4. Result:
```sh
Running Suite: Controller Suite - /Users/parham/Desktop/Email-v1/internal/controller
====================================================================================
Random Seed: 1717294153

Will run 10 of 10 specs
••••••••••

Ran 10 of 10 Specs in 7.869 seconds
SUCCESS! -- 10 Passed | 0 Failed | 0 Pending | 0 Skipped
PASS

Ginkgo ran 1 suite in 14.503898458s
Test Suite Passed
```
The test results are also uploaded in `/internal/controller/report.json` and `/internal/controller/report.xml`.

Test cases include:

- validation of the emailsenderconfig in both cases on valid and invalid emailsenderconfig.
- validation of the emailsenderconfig in multiple namespaces.
- send email using multiple providers in different namespaces.
- test if any invalid data (like invalid token, missing secret, missing senderconfig, invalid recipient email format, ...) is handled successfully.
- checking if the status and possible errors of emails are recorded properly.

## Updating the Operator

If you make changes to the CRDs, APIs, or operator code, you need to follow these steps:

   ```sh
   make uninstall
   make undeploy
   make
   make manifests
   ```

Then, install the CRDs and deploy the Operator as explained before.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request on GitHub.

## Contact

For any questions or feedback, please contact the project maintainer at [parham.doustkani@gmail.com](mailto:parham.dskn@gmail.com).
