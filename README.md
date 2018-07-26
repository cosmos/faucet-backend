# F11

## Overview

F11 is a faucet API backend implemented as an AWS Lambda function. Documentation is pending, together with a few critical fixes. Check back in a few days for an updated repository.

Beware: you'll have a hard time compiling this code without help. Wait for the documentation and cleanup.

## Todo
Main todo list from the code:

- config.go: Make sequence number increase thread-safe
- defaults.go: Put them into environment variables on Lambda so they are not baked into the code.
- main.go: Add lambda timeout function so a response is made before the function times out in AWS.
- middleware.go: Let the API Gateway handle CORS, instead of handling it in code.
- middleware.go: Better define IP throttling requirements and storage
- v1.go: Implement account check for enough coins
- v1_test.go: Implement V1ClaimHandler testing again. The first version was created when the complete implementation wasn't ready yet.

Unmentioned todo:
- More tests
- Waaay more documentation
- Get away from AWS sam and create a decent CloudFormation template instead
