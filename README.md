# Vertex Relay

This is a relay based on [Khatru](https://github.com/fiatjaf/khatru) for running Vertex DVMs and storing related events.

## DVM services

These services follow the NIP-90 DVM spec. The `i` (input field) is never used, only `param`s.

### Requests Parameters

| Parameter   | Description                                                                                                                                                    | Default Value                            | Maximum Value      |
|-------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------|--------------------|
| `source`    | Pubkey used for Personalized Pagerank, so it only applies when `sort` is set to `personalized`.                                                                | The pubkey signing the DVM request.      | -                  |
| `target`    | Pubkey the requester is interested in. Can be supplied multiple times for services that require it.                                                            | -                                        | -                  |
| `distance`  | Maximum (or minimum) number of hops in the graph to perform the calculation to (or from).                                                                      | `0`                                      | `5`                |
| `sort`      | Algorithm used to sort results. It must be either `personalized` (Personalized Pagerank) or `global` (Global Pagerank).                                        | `global`                                 | -                  |
| `limit`     | Maximum number of results returned in a response.                                                                                                              | `5` (or same as inputs when applicable). | `1000`             |
| `proofs`    | Whether to return applicable events (kinds 0, 3, etc.) on the websocket connection for clients to validate claims.                                             | `false`                                  | -                  |

All parameters except `target` are optional for all services. All pubkeys can be expressed in either hex or npub format.

### Responses

The response is included in the `content` field as escaped JSON.

**Fields:**
  - `pubkey` (a hex nostr public key)
  - `rank` (the rank of the pubkey, computed using the algorithm specified in the `sort` parameter, _relative_ to `source`)
  - `warning` and `candidate` flags (see kind 5315)
----

### 5312: Relevant Who Follow
- **Description**: returns a list of pairs `pubkey`:`rank` where each `pubkey` follows `target`.
- **Useful for**: Giving users information to assess the reputation of `target`.
- **Required parameters**: `target`

#### Example request

```json
{
  "id": "1cd2c73f53e602ae6f081997962bd43c730a565053080ab27ef7efb7335f7f49",
  "pubkey": "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
  "created_at": 1732758297,
  "kind": 5312,
  "tags": [
    [
      "param",
      "source",
      "npub10xlxvlhemja6c4dqv22uapctqupfhlxm9h8z3k2e72q4k9hcz7vqpkge6d"
    ],
    [
      "param",
      "target",
      "npub12ztlnw9a86ancfq2dgxft00jf532zqs3rq0epw3fcswrrjyfhg9qcavenc"
    ],
    [
      "param",
      "sort",
      "personalized"
    ],
    [
      "param",
      "limit",
      "5"
    ],
    [
      "param",
      "proofs",
      "false"
    ]
  ],
  "content": "",
  "sig": "22f8aa10a0a3e9ef44f2b6a050868f46f19fcc1bbd9da3c3b291164405fb854a4b83524770d82d008a7415636554defcfb5ea52bf42e8a681a69ef10a81bc8e2"
}
```

#### Example response

```json
{
  "id": "26594511e04ee1b20b94719616a2380b3dcaf0430e2fd6d4dcf59d24f9175fca",
  "pubkey": "a9b008476119ea693cbd2f0b4de99fd346e2e30880b18d42234a1158bd323783",
  "created_at": 1732758298,
  "kind": 6312,
  "tags": [
    [
      "e",
      "1cd2c73f53e602ae6f081997962bd43c730a565053080ab27ef7efb7335f7f49"
    ],
    [
      "p",
      "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
    ]
  ],
  "content": "[{\"pubkey\":\"bd0c0615960ff21214aee7f5b06fa0a104585938c8eb4b5cd4e2b109041fdf62\",\"rank\":0.0025},{\"pubkey\":\"d05ab982e1105476ab68e4c6728d148f8e6222154e60cc359ef6b8599c820bea\",\"rank\":0.00163},{\"pubkey\":\"6efd1b46b3e6d1ec2447af7c905827bc83e1330bee2c3a6a5b8e0769734785e2\",\"rank\":0.00154},{\"pubkey\":\"bb17f1e4e516e75e82a5b5e81c0120ffeb24e9e92866962440b9888ae82e42a1\",\"rank\":0.00111},{\"pubkey\":\"5097f9b8bd3ebb3c240a6a0c95bdf24d22a10211181f90ba29c41c31c889ba0a\",\"rank\":0.000107}]",
  "sig": "3c25ff7f8d6d847775a9aafb8b1f28d2f2e9b53f78de7f53b49fbbe46402358dc281be263c20919a426cbea86fbe9d36951fd5dd86465181d9d49be056616f53"
}
```

### 5313: Recommended Follows

- **Description**: returns a list of pairs `pubkey`:`rank` where `pubkey` is a recommendation for `source`. They are the pubkeys with the highest ranks excluding `source` and its follows.
- **Useful for**: Offering users recommendations on accounts they may want to follow.
- **Required parameters**: none.

#### Example request

This example uses no parameters, gets recommended follows for the signer.

```json
{
  "id": "588d828025eab6404ed17c6c7a70d09a67c5da4ffe780e2f943f32509fe8af23",
  "pubkey": "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
  "created_at": 1732759754,
  "kind": 5313,
  "tags": [],
  "content": "",
  "sig": "e0174ad1416e0d722f06491909fe8e4781fd732c21df6424cf0f1dc422db53ba525d544a29927297f63543796750ed7abf5e3c10c0e40e72b8c916b9a751c078"
}
```

#### Example response

```json
{
  "id": "171a0a7551c785ab0e2ac99577bfc25dd4fe7c28a19cacfcb625b4be2964ea4a",
  "pubkey": "a9b008476119ea693cbd2f0b4de99fd346e2e30880b18d42234a1158bd323783",
  "created_at": 1732759756,
  "kind": 6313,
  "tags": [
    [
      "e",
      "588d828025eab6404ed17c6c7a70d09a67c5da4ffe780e2f943f32509fe8af23"
    ],
    [
      "p",
      "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
    ]
  ],
"content": "[{\"pubkey\":\"bd0c0615960ff21214aee7f5b06fa0a104585938c8eb4b5cd4e2b109041fdf62\",\"rank\":0.0025},{\"pubkey\":\"d05ab982e1105476ab68e4c6728d148f8e6222154e60cc359ef6b8599c820bea\",\"rank\":0.00163},{\"pubkey\":\"6efd1b46b3e6d1ec2447af7c905827bc83e1330bee2c3a6a5b8e0769734785e2\",\"rank\":0.00154},{\"pubkey\":\"bb17f1e4e516e75e82a5b5e81c0120ffeb24e9e92866962440b9888ae82e42a1\",\"rank\":0.00111},{\"pubkey\":\"5097f9b8bd3ebb3c240a6a0c95bdf24d22a10211181f90ba29c41c31c889ba0a\",\"rank\":0.000107}]",
  "sig": "c79f34b9f5603b242e00f0b04782d579ffcec2cb45e511fbbf1ba3e04d5297f7eb7a071433b0a14300fbd766feaf5e8e1f6fbd216ae1cce1cb400f987fc2d0d2"
}
```

### 5314: Sort Authors

- **Description**: returns a list of pairs `target`:`rank`, one pair for each of the provided targets. 
- **Useful for**: Sorting comments under a note, zaps, and for improving search results and discovery.
- **Required parameters**: at least one `target`.

#### Example request

```json
{
  "id": "588d828025eab6404ed17c6c7a70d09a67c5da4ffe780e2f943f32509fe8af23",
  "pubkey": "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
  "created_at": 1732760317,
  "kind": 5314,
  "tags": [
    [
      "param",
      "target",
      "d05ab982e1105476ab68e4c6728d148f8e6222154e60cc359ef6b8599c820bea"
    ],
    [
      "param",
      "target",
      "6efd1b46b3e6d1ec2447af7c905827bc83e1330bee2c3a6a5b8e0769734785e2"
    ],
    [
      "param",
      "target",
      "bd0c0615960ff21214aee7f5b06fa0a104585938c8eb4b5cd4e2b109041fdf62"
    ],
    [
      "param",
      "sort",
      "personalized"
    ]
  ],
  "content": "",
  "sig": "be8b89b9db5f3579efe55417fbb76f626242936b3745aa0aaa67ed5a7e0107c7caa9a96bd1e78521528b642f240d972dcec88d6655992a80980a9acfd0c9ce72"
}
```

#### Example response

```json
{
  "id": "cc6a095e8e87977971806acea0670d92af3632da6242699c4a004ebad11b1347",
  "pubkey": "a9b008476119ea693cbd2f0b4de99fd346e2e30880b18d42234a1158bd323783",
  "created_at": 1732760389,
  "kind": 6314,
  "tags": [
    [
      "e",
      "588d828025eab6404ed17c6c7a70d09a67c5da4ffe780e2f943f32509fe8af23"
    ],
    [
      "p",
      "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
    ]
  ],
  "content": "[{\"pubkey\":\"bd0c0615960ff21214aee7f5b06fa0a104585938c8eb4b5cd4e2b109041fdf62\",\"rank\":0.0025},{\"pubkey\":\"d05ab982e1105476ab68e4c6728d148f8e6222154e60cc359ef6b8599c820bea\",\"rank\":0.00163},{\"pubkey\":\"6efd1b46b3e6d1ec2447af7c905827bc83e1330bee2c3a6a5b8e0769734785e2\",\"rank\":0.00154},{\"pubkey\":\"bb17f1e4e516e75e82a5b5e81c0120ffeb24e9e92866962440b9888ae82e42a1\",\"rank\":0.00111},{\"pubkey\":\"5097f9b8bd3ebb3c240a6a0c95bdf24d22a10211181f90ba29c41c31c889ba0a\",\"rank\":0.000107}]",
  "sig": "6fd60b9c07eac7b9150c25c4d5bb2652998b671b3b336c1407cac0473f90a25bfae5636a4eb27bcf40d2ba6f0b5f25e3300d3fdbae295dc9f2fc5cf74b793c11"
}
```

### 5315: Impersonator Detection (WIP)

#### Example request

```json
{
  "id": "79f8c1e8a4f8cb22fb10ee72fe643ad55ab179940d62a6b6b0d39dda46a9a80f",
  "pubkey": "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
  "created_at": 1732760561,
  "kind": 5315,
  "tags": [
    [
      "param",
      "target",
      "npub12ztlnw9a86ancfq2dgxft00jf532zqs3rq0epw3fcswrrjyfhg9qcavenc"
    ]
    [
      "param",
      "target",
      "6efd1b46b3e6d1ec2447af7c905827bc83e1330bee2c3a6a5b8e0769734785e2"
    ]
  ],
  "content": "",
  "sig": "1500fb21f44344a72c71758d4b4bd333197125e17d77d1af015432d9e638e2106fe91f3d479b0167b69195fb28fdac7bc05d97748462190b389227e3635e0fae"
}
```

#### Example response

```json
{
  "id": "4e372183ee58dda42396a5cc5290d5563847f93e3b8c79430af7eb7a67fac314",
  "pubkey": "a9b008476119ea693cbd2f0b4de99fd346e2e30880b18d42234a1158bd323783",
  "created_at": 1732760562,
  "kind": 6315,
  "tags": [
    [
      "e",
      "79f8c1e8a4f8cb22fb10ee72fe643ad55ab179940d62a6b6b0d39dda46a9a80f"
    ],
    [
      "p",
      "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
    ]
  ],
  "content": "[{\"pubkey\":\"2447af7c905827bc83e1330bee26efd1b46b3e6d1ecc3a6a5b8e0769734785e2\",\"rank\":0.0043,\"warning\":false,},{\"pubkey\":\"6efd1b46b3e6d1ec2447af7c905827bc83e1330bee2c3a6a5b8e0769734785e2\",\"rank\":0.000001,\"warning\":true,\"candidate\":d05ab982e1105476ab68e4c6728d148f8e6222154e60cc359ef6b8599c820bea}]",
  "sig": "1d351fe8bfb145beba13c72ca10e46b6aaca5e9b49c4503f51a34a33a19a54ae2d99ab680b628574e8666d3a867bbff354e7334b6e275c5a0537ff9f3dd0ade8"
}
```

### 5316: Degrees of Separation (WIP)

#### Example request

```json
{
  "id": "1d01554bb248102bcdb2d81dea08d9e000a745d98e1b39a80e47f04bddb929e5",
  "pubkey": "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
  "created_at": 1732760727,
  "kind": 5316,
  "tags": [
    [
      "param",
      "target",
      "npub12ztlnw9a86ancfq2dgxft00jf532zqs3rq0epw3fcswrrjyfhg9qcavenc"
    ]
  ],
  "content": "",
  "sig": "9c01144a93c49ceb9595ee87ddbebaa3b126f00ca7aad82b2155af11256a82e228ae296492549d4fd677b02a6855c52866a65089a81f4607e5e9ddd05a09568e"
}
```

#### Example response

```json
{
  "id": "e3f869d0c0b31b49568d7080e707375f89a55399623bb96b94bc5f414fd613ab",
  "pubkey": "a9b008476119ea693cbd2f0b4de99fd346e2e30880b18d42234a1158bd323783",
  "created_at": 1732760728,
  "kind": 6316,
  "tags": [
    [
      "e",
      "1d01554bb248102bcdb2d81dea08d9e000a745d98e1b39a80e47f04bddb929e5"
    ],
    [
      "p",
      "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
    ]
  ],
  "content": "3",
  "sig": "860539357ed5a471b263b7f30ef30999b1d92c471104e122c449ae342a7948c3115157288c33069accf185f1d316fcee4d8bd8eee49e4823b94fb98a86a41ee6"
}
```

### Errors

Errors are returned via kind 7000 for all services.

```json
{
  "id": "3910dd75eac0bc2099c45c697246584d09ad388da183c3b5546ccd1679a8478f",
  "pubkey": "a9b008476119ea693cbd2f0b4de99fd346e2e30880b18d42234a1158bd323783",
  "created_at": 1732760229,
  "kind": 7000,
  "tags": [
    [
      "e",
      "c7a59f2876008a30f8c08346addd8c7b4ebba93c4df9a0d75fc834ebaa927775"
    ],
    [
      "p",
      "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
    ],
    [
      "status",
      "error",
      "error decoding target key: npub1hq7rc8dpegj3ndf82c3ks2sk40dxt7qulx3klkk3vrzme455yh9rl2jsvt"
    ]
  ],
  "content": "",
  "sig": "35f8029687bbaac243f92eeba5c413d8a10bc55df2c281dcb0954f72533942dcec5f75af91cf452d35ee1551ce8576f5a746d6b03a9f8434f8a4d25719bdad7a"
}
```

## License

MIT; for more information, [check out the license](https://github.com/vertex-lab/relay/blob/master/LICENSE.md).
