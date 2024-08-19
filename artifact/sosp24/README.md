# SigmaOS SOSP24 Artifact

This README describes how to reproduce the results in our SOSP24 paper.

## Required Software/Hardware

Some of the paper experiments are run on AWS, and others are run on CloudLab.
Experiments performed on AWS require fast S3 access, whereas experiments
performed on CloudLab are tail-latency focused, and require kernel and hardware
configuration which, at the time of writing, is disallowed on AWS VMs. 
This section describes the Software and Hardware setup used in each setting.

### AWS

#### Summary of required resources

- 8 [m6i.4xlarge](https://aws.amazon.com/ec2/instance-types/m6i/) VMs
- 24 [t3.xlarge](https://aws.amazon.com/ec2/instance-types/t3/) VMs

#### Setup for each experiment

- Start-latency experiment
  - Number of VMs: 8
  - VM type: [m6i.4xlarge](https://aws.amazon.com/ec2/instance-types/m6i/)
    - 16vCPU, 2.9GHz Intel Ice Lake 8375C
    - 64GiB memory
    - 12.5Gbps network burst bandwitdh
    - 10Gbps EBS burst bandwidth
    - 20GB EBS storage
  - OS: Ubuntu Jammy 22.04

- Map Reduce experiment
  - Number of VMs: 8
  - VM type: [t3.xlarge](https://aws.amazon.com/ec2/instance-types/t3/)
    - 4vCPU (with 2vCPU disabled on each machine to make comparison to AWS
      Lambda fair)
    - 16GiB memory
    - 5Gbps network burst bandwidth
    - 2,085 Mbps EBS burst bandwidth
    - 20GB EBS storage
  - OS: Ubuntu Jammy 22.04

- Multi-realm image resize experiment
  - Number of VMs: 24
  - VM type: [t3.xlarge](https://aws.amazon.com/ec2/instance-types/t3/)
    - 4vCPU
    - 16GiB memory
    - 5Gbps network burst bandwidth
    - 2,085 Mbps EBS burst bandwidth
    - 20GB EBS storage
  - OS: Ubuntu Jammy 22.04

#### AWS Lambda experiments

### CloudLab

#### Summary of required resources

- 24 [c220g5](https://docs.cloudlab.us/hardware.html) nodes

#### Setup for each experiment

- Maximum spawn throughput experiment
  - Number of nodes: 24
  - Node configuration:
    - 40 logical cores
      - 2 Intel Xeon Silver 4114 10-core CPUs at 2.2GHz
    - 192GB ECC DDR4-2666 memory
    - Dual-port Intel X520DA2 10Gb NIC
    - 2,085 Mbps EBS burst bandwidth
    - 1TB 7200RPM 6G SAS HD
  - OS: Ubuntu Jammy 22.04

- Hotel and Socialnet application performance experiments
  - Number of nodes: 8
  - Node configuration:
    - 40 logical cores (8 physical cores enabled)
      - 2 Intel Xeon Silver 4114 10-core CPUs at 2.2GHz
    - 192GB ECC DDR4-2666 memory
    - Dual-port Intel X520DA2 10Gb NIC
    - 2,085 Mbps EBS burst bandwidth
    - 1TB 7200RPM 6G SAS HD
  - OS: Ubuntu Jammy 22.04

- Hotel and ImgResize multi-realm multiplexing experiment
  - Number of nodes: 8
  - Node configuration:
    - 40 logical cores (8 physical cores enabled)
      - 2 Intel Xeon Silver 4114 10-core CPUs at 2.2GHz
    - 192GB ECC DDR4-2666 memory
    - Dual-port Intel X520DA2 10Gb NIC
    - 2,085 Mbps EBS burst bandwidth
    - 1TB 7200RPM 6G SAS HD
  - OS: Ubuntu Jammy 22.04
