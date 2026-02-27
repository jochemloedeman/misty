# Misty design

Monitors are configured by users. A Monitor has a Location (value object with name and coordinates) and a DailyWindow (the recurring time-of-day range the user cares about). Forecasts are fetched per Monitor and contain weather variables for a specific timestamp. The application has a global ForecastHorizon: the number of hours ahead of now for which Forecasts are fetched. The time range [now, now + ForecastHorizon] is called the ForecastWindow.

The forecast cycle runs periodically and has two phases:

1. For each active Monitor, fetch weather data for the Monitor's Location covering the ForecastWindow. Upsert Forecasts for that Monitor.
2. For each active Monitor, filter its Forecasts to those within both the Monitor's DailyWindow and the ForecastWindow. Determine whether the filtered Forecasts indicate fog. Reconcile with the Monitor's existing Alert (create, update, or delete).

# Alert reconciliation rules

- When a Monitor is inactive, Alerts cannot be generated for it.
- There can only be a single active Alert per Monitor.
- Forecasts for a given Monitor and timestamp are updated while the timestamp lies within the ForecastWindow.
- An Alert's Start/End can be shortened or lengthened when its Forecasts are updated.
- An Alert is deleted when its Forecasts are updated and no longer predict fog.
- An Alert is deleted when its timeframe no longer overlaps with the ForecastWindow.
- An Alert's timeframe must overlap with the Monitor's DailyWindow.
- An Alert spans from the earliest to the latest foggy Forecast timestamp; gaps between foggy Forecasts within that range do not split the Alert.

# Considerations
- The model must be
    - as simple as possible
    - provide a good user experience. This means that a user is not bombarded with alerts, but also certainly alerted when foggy conditions occur.
    - Be flexible/expressive enough to be faithful to the weather conditions