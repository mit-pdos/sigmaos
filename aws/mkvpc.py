#!/usr/bin/env python
import argparse
import os
import sys

import boto3

parser=argparse.ArgumentParser(description='mk VPC on AWS.')
parser.add_argument('--vpc', metavar='vpc-id', help='Create only vm instance')
parser.add_argument('name', help='name for this VPC/instance')
parser.add_argument('--instance_type', type=str, default='t3.small')
args = vars(parser.parse_args())

#
# Create a Virtual Private Cloud (VPC).  If vpc is specified, then
# make only an EC2 instance.
#
# To find the IP address for ingress, curl https://checkip.amazonaws.com
#

def setup_net(ec2, vpc):
    # enable public dns hostname so that we can SSH into it later
    ec2Client = boto3.client('ec2')
    ec2Client.modify_vpc_attribute(VpcId = vpc.id, EnableDnsSupport = { 'Value': True } )
    ec2Client.modify_vpc_attribute(VpcId = vpc.id, EnableDnsHostnames = { 'Value': True } )
    # create an internet gateway and attach it to VPC
    ig = ec2.create_internet_gateway()
    vpc.attach_internet_gateway(InternetGatewayId=ig.id)

    # create a route table and a public route
    rt = vpc.create_route_table()
    route = rt.create_route(DestinationCidrBlock='0.0.0.0/0', GatewayId=ig.id)

    # create subnet and associate it with route table
    sn = ec2.create_subnet(CidrBlock='10.0.0.0/16', VpcId=vpc.id,AvailabilityZone='us-east-1c')
    rt.associate_with_subnet(SubnetId=sn.id)
    return sn

# Create a security group and allow SSH inbound rule through the VPC
def setup_sec_public(ec2, vpc, name):
    sg = ec2.create_security_group(GroupName=name, Description='Allow inbound traffic', VpcId=vpc.id)
    sg.authorize_ingress(CidrIp='18.26.0.0/16', IpProtocol='tcp', FromPort=22, ToPort=22,)
    sg.authorize_ingress(CidrIp='128.52.0.0/16', IpProtocol='tcp', FromPort=22, ToPort=22,)
    sg.authorize_ingress(CidrIp='173.76.107.0/24', IpProtocol='tcp', FromPort=22, ToPort=22,)
    sg.authorize_ingress(CidrIp='66.92.71.0/24', IpProtocol='tcp', FromPort=22, ToPort=22,)
    sg.authorize_ingress(CidrIp='75.100.81.0/24', IpProtocol='tcp', FromPort=22, ToPort=22,)
    sg.authorize_ingress(CidrIp='65.96.172.0/24', IpProtocol='tcp', FromPort=22, ToPort=22,)
    sg.authorize_ingress(CidrIp='10.0.0.0/16', IpProtocol='tcp', FromPort=0, ToPort=65535,)
    return sg
    
def kpname(vpc):
    return  'key-%s' % vpc.id

def setup_keypair(vpc, ec2):
    kpn = kpname(vpc)
    n = kpn+'.pem'
    outfile = open(n, 'w')
    kp = ec2.create_key_pair(KeyName=kpn)
    outfile.write(str(kp.key_material))
    os.chmod(n, 0o400)
    return kpn

def setup_instance(ec2, vpc, sg, sn, kpn, instance_type):
    script=''
    with open('cloud-localds-user-data', 'r') as fin:
        script = fin.read()

    instance = instance_type
    storage = 20
        
    vm = ec2.create_instances(
        ImageId='ami-09d56f8956ab235b3',
        InstanceType=instance,
        BlockDeviceMappings=[
            {
                'DeviceName': '/dev/sda1',
                'Ebs': {
                    'VolumeSize': storage,
                    'VolumeType': 'gp2'
                }
            }
        ],
        MaxCount=1,
        MinCount=1,
        NetworkInterfaces=[{
            'SubnetId': sn.id,
            'DeviceIndex': 0,
            'AssociatePublicIpAddress': True,
            'Groups': [sg.group_id]
        }],
        KeyName=kpn,
        UserData=script,
    )
    for i in vm:
        i.create_tags(Tags=[{"Key": "Name", "Value": "%s" % args['name']}])
    ec2Client = boto3.client('ec2')
    waiter = ec2Client.get_waiter('instance_running')
    waiter.wait(InstanceIds=[x.id for x in vm])

def find_sn(vpc):
    for sn in vpc.subnets.all():
        if sn.cidr_block == "10.0.0.0/16":
            return sn
    return None

def find_sg(vpc):
    for sg in vpc.security_groups.all():
        if sg.group_name == 'public2':
            return sg
    return None

def main():
    boto3.setup_default_session(profile_name='me-mit')
    ec2 = boto3.resource('ec2')
    if args['vpc'] == None:
        vpc = ec2.create_vpc(CidrBlock='10.0.0.0/16')
        vpc.wait_until_available()
        print(vpc.id)
        vpc.create_tags(Tags=[{"Key": "Name", "Value": "%s" % args['name']}])
        sn = setup_net(ec2, vpc)
        sg = setup_sec_public(ec2, vpc, "public2")
        kpn = setup_keypair(vpc, ec2)
        setup_instance(ec2, vpc, sg, sn, kpn, args['instance_type'])
    else:
        try:
            vpc = ec2.Vpc(args['vpc'])
        except Exception as e:
            print("error", e)
            sys.exit(0)
        sn = find_sn(vpc)
        sg = find_sg(vpc)
        kpn = kpname(vpc)
        setup_instance(ec2, vpc, sg, sn, kpn, args['instance_type'])

if __name__ == "__main__":
    main()
