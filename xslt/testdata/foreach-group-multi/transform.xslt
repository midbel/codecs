<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<cities>
			<xsl:for-each-group select="/cities/city" group-by="country-code, star">
				<country>
					<code>
						<xsl:value-of select="current-grouping-key()[1]"/>
					</code>
					<star>
						<xsl:value-of select="current-grouping-key()[2]"/>
					</star>
					<cities>
						<xsl:for-each select="current-group()">
							<city>
								<xsl:value-of select="name"/>
							</city>
						</xsl:for-each>
					</cities>
				</country>
			</xsl:for-each-group>
		</cities>
	</xsl:template>
</xsl:stylesheet>