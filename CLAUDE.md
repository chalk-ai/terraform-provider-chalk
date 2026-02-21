This repository holds a terraform provider for chalk.ai.

Chalk provides a gRPC interface (exposed via chalk-go, ../chalk-go) with protos defined by ../chalk-private/protos/. 

Most of the operations the terraform provider should use are in the 'chalk/server/' proto package, especially 'builder.proto' and 'deploy.proto' and 'team.proto'.

For testing, the testserver implementation is in ../chalk-go/testserver and can be updated as needed.


PROTO DEFINITIONS ARE LOCATED AT: ../chalk-private/protos/
