# Cassandra Reaper Operator
A Kubernetes operator for [Cassandra Reaper](http://cassandra-reaper.io/)

**Project status: alpha**

## Features
* Support for Cassandra storage backend
* Configure Reaper instance through `Reaper` custom resource
* Support for specifying resource requirements, e.g., cpu, memory
* Support for specifying affinity and anti-affinity

## Requirements
* Go >= 1.13.0
* Docker client >= 17
* kubectl >= 1.13
* Kubernetes >= 1.15.0
* [Operator SDK](https://github.com/operator-framework/operator-sdk) = 0.14.0

**Note:** The operator will work with earlier versions of Kubernetes, but the configuration update functionality requires >= 1.15.0.