## 动机

- 死了妈的 [Etherscan 系浏览器](https://etherscan.io/) 的 [Nametags API](https://docs.etherscan.io/etherscan-v2/api-endpoints/nametags) 居然他妈了个逼的好意思要钱, 而且只有最贵的那一档订阅才能调用, 早该埋土里的后端渲染上世纪垃圾也配张嘴要钱? 这傻逼玩意还套 cf 五秒盾, 爬都爬不了, 所以我个人认为五秒盾不应该套在网页上, 应该穿越回 50 年前套 Matthew Tan 他爹的 JB 上, 这样的话这个王八草的傻逼就不会出生了
- 为满足一毛钱也不用掏的本地查询需求, AddressInf0DB 支持对 [Flipside](https://docs.flipsidecrypto.xyz/data/flipside-data/labels) 地址数据库的**不完整爬取**与增量更新(排除了可以在链上拿到信息的合约, 大幅缩小数据库体积)

## 数据库信息

- 维护时间: ***2025-05-13***
- 文件大小: ***2.73 MB (2,867,200 字节)***
- Mainnet: ***22198***
- BSC: ***3144***
- Mainnet: ***22198***
- Base: ***195***

**数据库的地址实际上少于 [Etherscan 系浏览器](https://etherscan.io/): 详见 [brianleect](https://github.com/brianleect) 的由于数据爬取困难而停更的项目 [etherscan-labels](https://github.com/brianleect/etherscan-labels), 如果你有能力爬取数据, 那么可以考虑在他的基础上进行开发, 又能省事了**

**数据库随缘维护, 请自行定期进行增量更新!!!**

## 完整爬取/增量更新

```golang
addressInf0DB, err := addressinf0db.Open("./addressInf0.db", "FLIPSIDE_API_KEY")
if err != nil {
    log.Fatalln(fmt.Sprintf("Failed to open: %s", err))
}

// UpdateSync
log.Println("UpdateSync will start in 3 seconds...")
time.Sleep(3 * time.Second)

addressInf0DB.UpdateSync(&addressinf0db.Logger{
    Info:    log.Println,
    Success: log.Println,
    Warning: log.Println,
    Error:   log.Println,
})

log.Println("UpdateSync done")
```

- 不存在的数据库被打开时将会自动创建
- 若你未在自己的项目中维护数据库, 进行增量更新需要将数据库复制到 ***cmd/addressInf0.db*** 并运行 ***cmd/main.go***
- 若你已在自己的项目中维护数据库, 推荐使用 **UpdateSync(logger *Logger)**, 在需要打印 log 时传入你的 logger

## 注意事项

- **请不要使用别人编译好的可执行文件, 务必自行编译!!!**
- **请不要使用别人编译好的可执行文件, 务必自行编译!!!**
- **请不要使用别人编译好的可执行文件, 务必自行编译!!!**

## 鸣谢

#### [Flipside Docs](https://docs.flipsidecrypto.xyz/welcome-to-flipside/data/choose-your-flipside-plan/free#:~:text=At%20Flipside%2C%20we%20believe%20access%20to%20onchain%20data%20in%20web3%20is%20a%20public%20good%2C%20because%20transparency%20can%27t%20exist%20without%20human%2Dreadable%20data.)

> At Flipside, we believe access to onchain data in web3 is a public good, because transparency can't exist without human-readable data.
> 
> That's why you can always query the entire Flipside database with SQL in your Studio for free.

- AddressInf0DB 的数据来源于 [Flipside](https://flipsidecrypto.xyz/)
- [Flipside](https://flipsidecrypto.xyz/) 允许免费计划用户访问数据库与 API, 正因如此 AddressInf0DB 才能诞生, 感谢 [Flipside](https://flipsidecrypto.xyz/) 的卓越贡献