create table tb_market_order
(
    id                bigint auto_increment primary key,
    orderId           varchar(80)    default '' not null,
    utxoId            varchar(80)    default '' not null,
    outValue          bigint         default 0  not null,
    assetId           varchar(80)    default '' not null,
    assetType         varchar(50)    default '' not null comment 'pins/ordinals',
    assetNumber       bigint         default 0  not null,
    orderState        int            default 0  not null comment '1-create, 2-cancel, 3-finish',
    sellerAddress     varchar(80)    default '' not null,
    sellerIp          varchar(64)    default '' not null,
    buyerAddress      varchar(80)    default '' not null,
    buyerIp           varchar(64)    default '' not null,
    sellPriceAmount   bigint         default 0  not null,
    sellPriceDecimal  int            default 0  not null,
    sellPriceCoin     varchar(64)    default '' not null,
    feeAmount         bigint         default 0  not null,
    feeRate           bigint         default 0  not null,
    content           varchar(1024)  default '' not null,
    preview           varchar(256)   default '' not null,
    detail            varchar(2048)  default '' not null,
    makerPsbt         TEXT  not null,
    takerPsbt         TEXT  not null,
    finalPsbt         TEXT  not null,
    txId              varchar(128)   default '' not null,
    dealTime          bigint         default 0  not null,
    blockHeight       bigint         default 0  not null,
    confirmationState int            default 0  not null comment '1-Unconfirmed, 2-Confirmed',
    timestamp         bigint         default 0  not null,
    version           int            default 0  not null,
    createTime        bigint         default 0  not null,
    updateTime        bigint         default 0  not null,
    state             int            default 0  not null,
    constraint tb_market_order_orderId_uindex
        unique (orderId)
);

create table tb_market_utxo
(
    id             bigint auto_increment primary key,
    utxoId         varchar(128) default '' not null,
    utxoType       int          default 0  not null comment '1-dummy600, 2-dummy1200',
    amount         bigint       default 0  not null,
    address        varchar(128) default '' not null,
    privateKeyHex  varchar(128) default '' not null,
    txId           varchar(128) default '' not null,
    `index`          bigint       default 0  not null,
    pkScript       varchar(128) default '' not null,
    usedState      int          default 0  not null comment '1-UsedNo, 2-UsedYes, 3-UsedErr, 4-UsedDel',
    usedTxId       varchar(128) default '' not null,
    orderId        varchar(128) default '' not null,
    sortIndex      bigint       default 0  not null,
    confirmStatus  int          default 0  not null comment '0-Unconfirmed, 1-Confirmed, 1000-RoadBack',
    fromOrderId    varchar(128) default '' not null,
    networkFeeRate int          default 0  not null,
    version        int          default 0  not null,
    timestamp      bigint       default 0  not null,
    createTime     bigint       default 0  not null,
    updateTime     bigint       default 0  not null,
    state          int          default 0  not null,
    constraint tb_market_utxo_utxoId_uindex
        unique (utxoId)
);


alter table tb_market_order
    add assetLevel bigint default 0 not null AFTER `assetNumber`;
alter table tb_market_order
    add assetPop varchar(64) default '' not null AFTER `assetLevel`;



create table tb_mrc20_mint_order
(
    id                  bigint auto_increment primary key,
    orderId             varchar(80)    default '' not null,
    inscribeState       int            default 0  not null comment '1-Pending,2-Paid,3-Finish',
    ticketId            varchar(80)    default '' not null,
    totalFee            bigint         default 0  not null,
    minerFee            bigint         default 0  not null,
    serviceFee          bigint         default 0  not null,
    revealOutValue      bigint         default 0  not null,
    redeemScript        varchar(2048)  default '' not null,
    controlBlockWitness varchar(2048)  default '' not null,
    revealTxPrivateKey  varchar(2048)  default '' not null,
    revealTxAddress     varchar(80)    default '' not null,
    commitTxRaw         TEXT,
    revealInputIndex    bigint         default 0  not null,
    revealPrePsbtRaw    TEXT,
    revealMidPsbtRaw    TEXT,
    revealFinalPsbtRaw  TEXT,
    commitTxId          varchar(128)   default '' not null,
    txId                varchar(128)   default '' not null,
    blockHeight         bigint         default 0  not null,
    confirmationState   int            default 0  not null comment '1-Unconfirmed, 2-Confirmed',
    timestamp           bigint         default 0  not null,
    version             int            default 0  not null,
    createTime          bigint         default 0  not null,
    updateTime          bigint         default 0  not null,
    state               int            default 0  not null,
    constraint tb_mrc20_mint_order_orderId_uindex
        unique (orderId)
);


create table tb_mrc20_transfer_order
(
    id                  bigint auto_increment primary key,
    orderId             varchar(80)    default '' not null,
    inscribeState       int            default 0  not null comment '1-Pending,2-Paid,3-Finish',
    ticketId            varchar(80)    default '' not null,
    payload            varchar(512)    default '' not null,
    totalFee            bigint         default 0  not null,
    minerFee            bigint         default 0  not null,
    serviceFee          bigint         default 0  not null,
    revealOutValue      bigint         default 0  not null,
    redeemScript        varchar(2048)  default '' not null,
    controlBlockWitness varchar(2048)  default '' not null,
    revealTxPrivateKey  varchar(2048)  default '' not null,
    revealTxAddress     varchar(80)    default '' not null,
    commitTxRaw         TEXT,
    revealInputIndex    bigint         default 0  not null,
    revealPrePsbtRaw    TEXT,
    revealMidPsbtRaw    TEXT,
    revealFinalPsbtRaw  TEXT,
    commitTxId          varchar(128)   default '' not null,
    txId                varchar(128)   default '' not null,
    blockHeight         bigint         default 0  not null,
    confirmationState   int            default 0  not null comment '1-Unconfirmed, 2-Confirmed',
    timestamp           bigint         default 0  not null,
    version             int            default 0  not null,
    createTime          bigint         default 0  not null,
    updateTime          bigint         default 0  not null,
    state               int            default 0  not null,
    constraint tb_mrc20_transfer_order_orderId_uindex
        unique (orderId)
);

create table tb_market_mrc20_order
(
    id                bigint auto_increment primary key,
    orderId           varchar(80)    default '' not null,
    utxoId            varchar(80)    default '' not null,
    assetType         varchar(50)    default '' not null comment 'mrc20',
    outValue          bigint         default 0  not null,
    tickId            varchar(80)    default '' not null,
    tick              varchar(80)    default '' not null,
    tokenName         varchar(80)    default '' not null,
    decimals          int            default 0  not null,
    chain             varchar(80)    default '' not null,
    amount            bigint         default 0  not null,
    amountStr         varchar(80)    default '' not null,
    tokenPriceRate    double         default 0  not null,
    tokenPriceRateStr varchar(80)    default '' not null,
    priceAmount       bigint         default 0  not null,
    priceDecimal      int            default 0  not null,
    priceCoin         varchar(80)    default '' not null,
    orderState        int            default 0  not null comment '1-create, 2-cancel, 3-finish',
    sellerAddress     varchar(80)    default '' not null,
    sellerIp          varchar(64)    default '' not null,
    buyerAddress      varchar(80)    default '' not null,
    buyerIp           varchar(64)    default '' not null,
    feeAmount         bigint         default 0  not null,
    feeRate           bigint         default 0  not null,
    makerPsbt         TEXT  not null,
    takerPsbt         TEXT  not null,
    finalPsbt         TEXT  not null,
    txId              varchar(128)   default '' not null,
    dealTime          bigint         default 0  not null,
    blockHeight       bigint         default 0  not null,
    confirmationState int            default 0  not null comment '1-Unconfirmed, 2-Confirmed',
    timestamp         bigint         default 0  not null,
    version           int            default 0  not null,
    createTime        bigint         default 0  not null,
    updateTime        bigint         default 0  not null,
    state             int            default 0  not null,
    constraint tb_market_mrc20_order_orderId_uindex
        unique (orderId)
);


alter table tb_mrc20_mint_order
    change ticketId tickId varchar(80) default '' not null;
alter table tb_mrc20_transfer_order
    change ticketId tickId varchar(80) default '' not null;
alter table tb_mrc20_mint_order
    add address varchar(80) default '' not null AFTER `inscribeState`;
alter table tb_mrc20_transfer_order
    add address varchar(80) default '' not null AFTER `inscribeState`;

create table tb_mrc20_deploy_order
(
    id                bigint auto_increment primary key,
    orderId           varchar(80)    default '' not null,
    inscribeState     int            default 0  not null comment '1-Pending,2-Paid,3-Finish',
    address           varchar(80)    default '' not null,
    tickId            varchar(80)    default '' not null,
    tick              varchar(80)    default '' not null,
    tokenName         varchar(80)    default '' not null,
    decimals          int            default 0  not null,
    amtPerMint        varchar(80)    default '' not null,
    mintCount         varchar(80)    default '' not null,
    premineCount      varchar(80)    default '' not null,
    startBlockHeight  varchar(80)    default '' not null,
    qual              varchar(512)    default '' not null,
    payload           varchar(512)   default '' not null,
    chain             varchar(80)    default '' not null,
    commitTxRaw       TEXT  not null,
    revealTxRaw       TEXT  not null,
    commitTxId        varchar(128)   default '' not null,
    revealTxId        varchar(128)   default '' not null,
    txId              varchar(128)   default '' not null,
    blockHeight       bigint         default 0  not null,
    confirmationState int            default 0  not null comment '1-Unconfirmed, 2-Confirmed',
    timestamp         bigint         default 0  not null,
    version           int            default 0  not null,
    createTime        bigint         default 0  not null,
    updateTime        bigint         default 0  not null,
    state             int            default 0  not null,
    constraint tb_mrc20_deploy_order_orderId_uindex
        unique (orderId)
);
