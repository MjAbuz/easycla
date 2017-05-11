variable "access_key" {
  description = "Your AWS Access Key"
}

variable "secret_key" {
  description = "Your AWS Secret Key"
}

variable "name" {
  description = "Name of your VPC"
}

variable "region" {
  description = "Region to launch this infra on"
}

variable "cidr" {
  description = "The CIDR block for the VPC."
}

variable "availability_zones" {
  description = "List of availability zones"
  type        = "list"
}

variable "external_subnets" {
  description = "List of external subnets"
  type        = "list"
}

variable "internal_subnets" {
  description = "List of internal subnets"
  type        = "list"
}

variable "key_name" {
  description = "Key Pair to use to administer this vpc"
}

variable "newrelic_key" {
  description = "Key to use for NewRelic"
}

variable "region_identitier" {
  description = "Label to recognize the region"
}

variable "pypi_redis_host" {
  description = "Redis host to use for Pypi Database"
}

variable "pypi_bucket" {
  description = "The bucket to use to store Pypi Repository files"
}

variable "consul_encryption_key" {
  description = "The encryption key used for consul"
}

variable "dns_server" {
  description = "DNS Server for the VPC"
}

variable "r53_zone_id" {}

variable "ghe_peering" {}

provider "aws" {
  region     = "${var.region}"
  alias      = "local"
  access_key = "${var.access_key}"
  secret_key = "${var.secret_key}"
}

module "vpc" {
  source             = "../../modules/vpc"
  name               = "${var.name}"
  cidr               = "${var.cidr}"
  internal_subnets   = "${var.internal_subnets}"
  external_subnets   = "${var.external_subnets}"
  availability_zones = "${var.availability_zones}"
}

module "dhcp" {
  source  = "../../modules/dhcp"
  name    = "prod.engineering.internal"
  vpc_id  = "${module.vpc.id}"
  servers = "${cidrhost(var.cidr, 2)}"
}

module "security_groups" {
  source  = "./security_groups"
  cidr    = "${var.cidr}"
  vpc_id  = "${module.vpc.id}"
  name    = "Engineering"
}

resource "aws_route53_zone_association" "prod_zone" {
  zone_id      = "${var.r53_zone_id}"
  vpc_id       = "${module.vpc.id}"
  vpc_region   = "${var.region}"
}

resource "aws_vpc_peering_connection" "peer" {
  provider      = "aws.local"
  count         = "${var.ghe_peering}"

  peer_owner_id = "961082193871"
  peer_vpc_id   = "vpc-10c9f477"
  vpc_id        = "${module.vpc.id}"

  accepter {
    allow_remote_vpc_dns_resolution = true
  }
}

resource "aws_route" "peer_internal_1" {
  provider                  = "aws.local"
  count                     = "${var.ghe_peering}"
  route_table_id            = "${module.vpc.raw_route_tables_id[0]}"
  destination_cidr_block    = "10.31.0.0/23"
  vpc_peering_connection_id = "${aws_vpc_peering_connection.peer.id}"
}

resource "aws_route" "peer_internal_2" {
  provider                  = "aws.local"
  count                     = "${var.ghe_peering}"
  route_table_id            = "${module.vpc.raw_route_tables_id[1]}"
  destination_cidr_block    = "10.31.0.0/23"
  vpc_peering_connection_id = "${aws_vpc_peering_connection.peer.id}"
}

resource "aws_route" "peer_internal_3" {
  provider                  = "aws.local"
  count                     = "${var.ghe_peering}"
  route_table_id            = "${module.vpc.raw_route_tables_id[2]}"
  destination_cidr_block    = "10.31.0.0/23"
  vpc_peering_connection_id = "${aws_vpc_peering_connection.peer.id}"
}

resource "aws_route" "peer_external" {
  provider                  = "aws.local"
  count                     = "${var.ghe_peering}"
  route_table_id            = "${module.vpc.external_rtb_id}"
  destination_cidr_block    = "10.31.0.0/23"
  vpc_peering_connection_id = "${aws_vpc_peering_connection.peer.id}"
}

data "template_file" "ecs_instance_cloudinit_tools" {
  template = "${file("${path.module}/cloud-config.sh.tpl")}"

  vars {
    ecs_cluster_name = "production-tools"
    newrelic_key     = "${var.newrelic_key}"
  }
}

module "tools-ecs-cluster" {
  source                 = "../../modules/ecs-cluster"
  environment            = "Production"
  team                   = "Engineering"
  name                   = "production-tools"
  vpc_id                 = "${module.vpc.id}"
  subnet_ids             = "${module.vpc.internal_subnets}"
  key_name               = "${var.key_name}"
  iam_instance_profile   = "arn:aws:iam::433610389961:instance-profile/ecsInstanceRole"
  region                 = "${var.region}"
  availability_zones     = "${module.vpc.availability_zones}"
  instance_type          = "t2.medium"
  security_group         = "${module.security_groups.tools-ecs-cluster}"
  instance_ebs_optimized = false
  desired_capacity       = "3"
  min_size               = "3"
  cloud_config_content   = "${data.template_file.ecs_instance_cloudinit_tools.rendered}"
}

module "consul" "consul-master" {
  source                 = "./consul"

  vpc_id                 = "${module.vpc.id}"
  ecs_cluster_name       = "${module.tools-ecs-cluster.name}"
  ecs_asg_name           = "${module.tools-ecs-cluster.asg_name}"
  internal_subnets       = "${module.vpc.internal_subnets}"
  internal_elb_sg        = "${module.security_groups.internal_elb}"
  consul_encryption_key  = "${var.consul_encryption_key}"
  r53_zone_id            = "${var.r53_zone_id}"
  region_identifier      = "${var.region_identitier}"
}

module "nginx" {
  source                 = "./nginx"

  vpc_id                 = "${module.vpc.id}"
  region                 = "${var.region}"

  ecs_cluster_name       = "${module.tools-ecs-cluster.name}"
  ecs_asg_name           = "${module.tools-ecs-cluster.asg_name}"
  internal_subnets       = "${module.vpc.internal_subnets}"
  internal_elb_sg        = "${module.security_groups.internal_elb}"
}

module "registrator" {
  source = "./registrator"

  ecs_cluster_name = "${module.tools-ecs-cluster.name}"
}

module "vault" "vault-master" {
  source                 = "./vault"

  ecs_cluster_name       = "${module.tools-ecs-cluster.name}"
  ecs_asg_name           = "${module.tools-ecs-cluster.asg_name}"
  internal_subnets       = "${module.vpc.internal_subnets}"
  internal_elb_sg        = "${module.security_groups.internal_elb}"
  consul_endpoint        = "${module.consul.consul_elb_cname}"
}

module "logstash" {
  source                 = "./logstash"

  ecs_cluster_name       = "${module.tools-ecs-cluster.name}"
  ecs_asg_name           = "${module.tools-ecs-cluster.asg_name}"
  internal_subnets       = "${module.vpc.internal_subnets}"
  internal_elb_sg        = "${module.security_groups.internal_elb}"

  vpc_id                 = "${module.vpc.id}"
  region                 = "${var.region}"
}

module "pypi" "pypi-master" {
  source                 = "./pypi"

  ecs_cluster_name       = "${module.tools-ecs-cluster.name}"
  ecs_asg_name           = "${module.tools-ecs-cluster.asg_name}"
  internal_subnets       = "${module.vpc.internal_subnets}"
  internal_elb_sg        = "${module.security_groups.internal_elb}"

  s3_bucket              = "${var.pypi_bucket}"
  redis_host             = "${var.pypi_redis_host}"

  vpc_id                 = "${module.vpc.id}"
  region                 = "${var.region}"
}

module "pritunl" {
  source                 = "./pritunl"

  external_subnets       = "${module.vpc.external_subnets}"
  vpn_sg                 = "${module.security_groups.vpn}"
  region_identifier      = "${var.region_identitier}"
}

/**
 * Outputs
 */


// The region in which the infra lives.
output "region" {
  value = "${var.region}"
}

// The VPC's CIDR
output "cidr" {
  value = "${var.cidr}"
}

// Comma separated list of internal subnet IDs.
output "internal_subnets" {
  value = "${module.vpc.internal_subnets}"
}

// Comma separated list of external subnet IDs.
output "external_subnets" {
  value = "${module.vpc.external_subnets}"
}

// The VPC availability zones.
output "availability_zones" {
  value = "${module.vpc.availability_zones}"
}

// The VPC security group ID.
output "vpc_security_group" {
  value = "${module.vpc.security_group}"
}

// The VPC ID.
output "vpc_id" {
  value = "${module.vpc.id}"
}

// The VPC security group ID.
output "raw_route_tables_id" {
  value = "${module.vpc.raw_route_tables_id}"
}

// The VPC ID.
output "external_rtb_id" {
  value = "${module.vpc.external_rtb_id}"
}

// Comma separated list of internal route table IDs.
output "internal_route_tables" {
  value = "${module.vpc.internal_rtb_id}"
}

// The external route table ID.
output "external_route_tables" {
  value = "${module.vpc.external_rtb_id}"
}

// External ELB allows traffic from the world.
output "sg_internal_elb" {
  value = "${module.security_groups.internal_elb}"
}

output "tools_ecs_asg" {
  value = "${module.tools-ecs-cluster.asg_name}"
}

output "tools_ecs_name" {
  value = "${module.tools-ecs-cluster.name}"
}

output "tools_ecs_sg" {
  value = "${module.tools-ecs-cluster.security_group_id}"
}

output "consul_elb_cname" {
  value = "${module.consul.consul_elb_cname}"
}

output "consul_elb_name" {
  value = "${module.consul.consul_elb_name}"
}

output "consul_service_name" {
  value = "${module.consul.consul_service_name}"
}

output "consul_elb_zoneid" {
  value = "${module.consul.consul_elb_zoneid}"
}

output "pypi_elb_cname" {
  value = "${module.pypi.pypi_elb_cname}"
}

output "pypi_elb_name" {
  value = "${module.pypi.pypi_elb_name}"
}

output "pypi_elb_zoneid" {
  value = "${module.pypi.pypi_elb_zoneid}"
}
