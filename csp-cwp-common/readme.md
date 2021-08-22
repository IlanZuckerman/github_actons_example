# csp-cwp-common
## cloud security platform - cloud workload protection common
This is the common repository for code that is shared between at least 2 repositories. 
Make sure you follow the right importing direction to avoid cyclic import.
please read carefully the following articles:
1. [go writing standarts](https://divvycloud.atlassian.net/wiki/spaces/EN/pages/10490904845/Go+writing+standards)
2. [working with go modules](https://divvycloud.atlassian.net/wiki/spaces/DEV/pages/10018651534/working+with+go+modules)

##key items:
* `Build` folder should contain anything related to build process
* `cmd` folder is for main function code and executable source code
* `examples` folder is for code examples and helpful main files to manual test your code
* `pkg` folder is for the go source code itself
