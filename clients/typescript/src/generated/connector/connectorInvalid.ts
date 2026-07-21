export interface IConnectorInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Connector:ConnectorInvalid";
    'parameters': {
        reason: string;
    };
}

export function isConnectorInvalid(arg: any): arg is IConnectorInvalid {
    return arg && arg.errorName === "Connector:ConnectorInvalid";
}
