# F11

## Overview

F11 is a faucet API backend implemented as an AWS Lambda function. Documentation is pending. Check back in a few days for an updated repository.

## Todo
Main todo list from the code:

- middleware.go: Let the API Gateway handle CORS, instead of handling it in code.
- middleware.go: Better define IP throttling requirements and storage
- v1.go: Implement account check for enough coins
- v1_test.go: Implement V1ClaimHandler testing again. The first version was created when the complete implementation wasn't ready yet.

Unmentioned todo:
- More tests
- Waaay more documentation

