<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<breakdown>
			<xsl:for-each-group select="/breakdown/item" group-by="rate">
				<item>
					<rate>
						<xsl:value-of select="current-grouping-key()"/>
					</rate>
					<total>
						<xsl:value-of select="sum(current-group()/total)"/>
					</total>
				</item>
			</xsl:for-each-group>
		</breakdown>
	</xsl:template>
</xsl:stylesheet>