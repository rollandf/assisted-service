import os
import utils
import yaml
import deployment_options

log = utils.get_logger('deploy_webhooks')

def main():
    log.info('Starting webhooks deployment')
    deploy_options = deployment_options.load_deployment_options()
    if deploy_options.enable_kube_api:
        utils.verify_build_directory(deploy_options.namespace)
        deploy(deploy_options, 'service.yaml')
        deploy(deploy_options, 'service-account.yaml')
        deploy(deploy_options, 'agentinstalladmission_rbac_role.yaml')
        deploy(deploy_options, 'agentclusterinstall-webhook.yaml')
        deploy_deployment(deploy_options)
        deploy_with_ns_replace(deploy_options, 'apiservice.yaml')
        deploy_with_ns_replace(deploy_options, 'agentinstalladmission_rbac_role_binding.yaml')
    log.info('Completed to webhooks deployment')


def deploy(deploy_options, name):
    docs = utils.load_yaml_file_docs(basename=f'deploy/webhooks/{name}')
    utils.set_namespace_in_yaml_docs(docs, deploy_options.namespace)

    dst_file = utils.dump_yaml_file_docs(
        basename=f'build/{deploy_options.namespace}/webhook-{name}',
        docs=docs
    )
    if not deploy_options.apply_manifest:
        return

    log.info('Deploying %s', dst_file)
    utils.apply(
        target=deploy_options.target,
        namespace=None,
        file=dst_file
    )

def deploy_deployment(deploy_options):
    src_file = os.path.join(os.getcwd(), 'deploy/webhooks/deployment.yaml')
    dst_file = os.path.join(os.getcwd(), 'build', deploy_options.namespace, 'webhook-deployment.yaml')

    with open(src_file, "r") as src:
        raw_data = src.read()
        raw_data = raw_data.replace('REPLACE_NAMESPACE', f'"{deploy_options.namespace}"')
        data = yaml.safe_load(raw_data)

        image_fqdn = deployment_options.get_image_override(deploy_options, "assisted-service", "SERVICE")
        data["spec"]["template"]["spec"]["containers"][0]["image"] = image_fqdn

        if deploy_options.image_pull_policy:
            data["spec"]["template"]["spec"]["containers"][0]["imagePullPolicy"] = deploy_options.image_pull_policy

    with open(dst_file, "w+") as dst:
        yaml.dump(data, dst, default_flow_style=False)

    if not deploy_options.apply_manifest:
        return

    log.info(f"Deploying {dst_file}")
    utils.apply(
        target=deploy_options.target,
        namespace=deploy_options.namespace,
        file=dst_file
    )

def deploy_with_ns_replace(deploy_options, name):
    src_file = os.path.join(os.getcwd(), f'deploy/webhooks/{name}')
    dst_file = os.path.join(os.getcwd(), 'build', deploy_options.namespace, f'webhook-{name}')

    with open(src_file, "r") as src:
        raw_data = src.read()
        raw_data = raw_data.replace('REPLACE_NAMESPACE', f'{deploy_options.namespace}')

    with open(dst_file, "w+") as dst:
        dst.write(raw_data)

    if not deploy_options.apply_manifest:
        return

    log.info(f"Deploying {dst_file}")
    utils.apply(
        target=deploy_options.target,
        namespace=deploy_options.namespace,
        file=dst_file
    )

if __name__ == "__main__":
    main()
