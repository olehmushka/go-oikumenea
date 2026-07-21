export interface IConnectorNotRegistered {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Wiring:ConnectorNotRegistered";
    'parameters': {
        principalId: string;
    };
}

export function isConnectorNotRegistered(arg: any): arg is IConnectorNotRegistered {
    return arg && arg.errorName === "Wiring:ConnectorNotRegistered";
}
